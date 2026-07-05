package discovery

import (
	"testing"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWorkloadFromSecretProviderClassExtractsSecretObjects(t *testing.T) {
	spc := newSecretProviderClassObject("payments", "payments-vault", []string{"payments-db", "payments-tls"})
	spc.SetUID("spc-uid")
	spc.SetAnnotations(map[string]string{
		AnnotationDiscoveryMode: "infer",
		AnnotationOwnerTeam:     "platform",
		AnnotationCriticality:   "high",
	})
	spc.SetLabels(map[string]string{
		"app.kubernetes.io/name": "payments-api",
		"environment":            "prod",
	})

	input := WorkloadFromSecretProviderClass(DefaultOptions("test-cluster"), spc)

	if input.Ref.Cluster != "test-cluster" ||
		input.Ref.Namespace != "payments" ||
		input.Ref.APIVersion != SecretsStoreSecretProviderClassAPIVersion ||
		input.Ref.Kind != SecretsStoreSecretProviderClassKind ||
		input.Ref.Name != "payments-vault" ||
		input.Ref.UID != "spc-uid" {
		t.Fatalf("unexpected SecretProviderClass ref: %#v", input.Ref)
	}

	if input.OwnerTeam != "platform" {
		t.Fatalf("expected owner team platform, got %q", input.OwnerTeam)
	}

	if input.Criticality != "high" {
		t.Fatalf("expected criticality high, got %q", input.Criticality)
	}

	if input.Service != "payments-api" {
		t.Fatalf("expected service label fallback, got %q", input.Service)
	}

	if input.Environment != "prod" {
		t.Fatalf("expected environment label fallback, got %q", input.Environment)
	}

	if len(input.Edges) != 2 {
		t.Fatalf("expected two SecretProviderClass edges, got %#v", input.Edges)
	}

	edges := map[string]graph.DependencyEdge{}
	for _, edge := range input.Edges {
		edges[graph.SecretID(edge.Secret)] = edge
	}

	first, ok := edges["payments/payments-db"]
	if !ok {
		t.Fatalf("expected payments-db edge, got %#v", input.Edges)
	}
	assertSecretProviderClassSource(t, first, "spec.secretObjects[0].secretName")

	second, ok := edges["payments/payments-tls"]
	if !ok {
		t.Fatalf("expected payments-tls edge, got %#v", input.Edges)
	}
	assertSecretProviderClassSource(t, second, "spec.secretObjects[1].secretName")
}

func TestWorkloadFromSecretProviderClassMergesExplicitAndInferredEdges(t *testing.T) {
	spc := newSecretProviderClassObject("payments", "payments-vault", []string{"payments-db"})
	spc.SetAnnotations(map[string]string{
		AnnotationDiscoveryMode: "hybrid",
		AnnotationWatchSecrets:  "payments-db,shared/vault-token",
	})

	input := WorkloadFromSecretProviderClass(DefaultOptions("test-cluster"), spc)

	edges := map[string]graph.DependencyEdge{}
	for _, edge := range input.Edges {
		edges[graph.SecretID(edge.Secret)] = edge
	}

	targetEdge, ok := edges["payments/payments-db"]
	if !ok {
		t.Fatalf("expected inferred target Secret edge, got %#v", input.Edges)
	}

	if targetEdge.DiscoveryMode != graph.DiscoveryModeHybrid {
		t.Fatalf("expected target edge mode hybrid after infer+explicit merge, got %q", targetEdge.DiscoveryMode)
	}

	if len(targetEdge.Sources) != 2 {
		t.Fatalf("expected merged target edge sources, got %#v", targetEdge.Sources)
	}

	if _, ok := edges["shared/vault-token"]; !ok {
		t.Fatalf("expected explicit cross-namespace vault token edge, got %#v", input.Edges)
	}
}

func TestWorkloadFromSecretProviderClassHonorsDisabledMode(t *testing.T) {
	spc := newSecretProviderClassObject("payments", "payments-vault", []string{"payments-db"})
	spc.SetAnnotations(map[string]string{
		AnnotationDiscoveryMode: "disabled",
		AnnotationWatchSecrets:  "shared/vault-token",
	})

	input := WorkloadFromSecretProviderClass(DefaultOptions("test-cluster"), spc)

	if input.DiscoveryMode != graph.DiscoveryModeDisabled {
		t.Fatalf("expected disabled mode, got %q", input.DiscoveryMode)
	}

	if len(input.Edges) != 0 {
		t.Fatalf("expected no edges in disabled mode, got %#v", input.Edges)
	}
}

func TestWorkloadFromSecretProviderClassWithoutSecretObjectsKeepsExplicitEdges(t *testing.T) {
	spc := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": SecretsStoreSecretProviderClassAPIVersion,
			"kind":       SecretsStoreSecretProviderClassKind,
			"metadata": map[string]any{
				"namespace": "payments",
				"name":      "payments-vault",
				"annotations": map[string]any{
					AnnotationDiscoveryMode: "hybrid",
					AnnotationWatchSecrets:  "shared/vault-token",
				},
			},
			"spec": map[string]any{},
		},
	}

	input := WorkloadFromSecretProviderClass(DefaultOptions("test-cluster"), spc)

	if len(input.Edges) != 1 {
		t.Fatalf("expected one explicit edge, got %#v", input.Edges)
	}

	if input.Edges[0].Secret.Namespace != "shared" || input.Edges[0].Secret.Name != "vault-token" {
		t.Fatalf("unexpected explicit edge: %#v", input.Edges[0])
	}
}

func TestWorkloadFromSecretProviderClassIgnoresBlankSecretObjects(t *testing.T) {
	spc := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": SecretsStoreSecretProviderClassAPIVersion,
			"kind":       SecretsStoreSecretProviderClassKind,
			"metadata": map[string]any{
				"namespace": "payments",
				"name":      "payments-vault",
			},
			"spec": map[string]any{
				"secretObjects": []any{
					map[string]any{"secretName": "payments-db"},
					map[string]any{"secretName": ""},
					map[string]any{"type": "Opaque"},
					"not-an-object",
				},
			},
		},
	}

	input := WorkloadFromSecretProviderClass(DefaultOptions("test-cluster"), spc)

	if len(input.Edges) != 1 {
		t.Fatalf("expected one valid SecretProviderClass edge, got %#v", input.Edges)
	}

	if input.Edges[0].Secret.Name != "payments-db" {
		t.Fatalf("unexpected SecretProviderClass edge: %#v", input.Edges[0])
	}
}

func newSecretProviderClassObject(namespace, name string, secretNames []string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": SecretsStoreSecretProviderClassAPIVersion,
			"kind":       SecretsStoreSecretProviderClassKind,
			"metadata": map[string]any{
				"namespace": namespace,
				"name":      name,
			},
			"spec": map[string]any{},
		},
	}

	secretObjects := make([]any, 0, len(secretNames))
	for _, secretName := range secretNames {
		secretObjects = append(secretObjects, map[string]any{
			"secretName": secretName,
			"type":       "Opaque",
			"data": []any{
				map[string]any{
					"objectName": "password",
					"key":        "password",
				},
			},
		})
	}

	if len(secretObjects) > 0 {
		_ = unstructured.SetNestedSlice(obj.Object, secretObjects, "spec", "secretObjects")
	}

	return obj
}

func assertSecretProviderClassSource(t *testing.T, edge graph.DependencyEdge, expectedPath string) {
	t.Helper()

	if edge.Secret.Cluster != "test-cluster" || edge.Secret.Namespace != "payments" {
		t.Fatalf("unexpected SecretProviderClass target Secret edge: %#v", edge.Secret)
	}

	if edge.DiscoveryMode != graph.DiscoveryModeInfer {
		t.Fatalf("expected inferred SecretProviderClass edge, got %q", edge.DiscoveryMode)
	}

	if edge.Optional {
		t.Fatalf("expected SecretProviderClass target Secret edge to be non-optional")
	}

	if len(edge.Sources) != 1 {
		t.Fatalf("expected one source, got %#v", edge.Sources)
	}

	source := edge.Sources[0]
	if source.Type != SourceSecretsStoreSecretProviderClassSecretObjects {
		t.Fatalf("expected SecretProviderClass source type %q, got %q", SourceSecretsStoreSecretProviderClassSecretObjects, source.Type)
	}

	if source.Path != expectedPath {
		t.Fatalf("expected source path %s, got %q", expectedPath, source.Path)
	}

	if source.ResourceField != expectedPath {
		t.Fatalf("expected resource field %s, got %q", expectedPath, source.ResourceField)
	}
}
