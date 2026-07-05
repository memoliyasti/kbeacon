package discovery

import (
	"testing"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWorkloadFromExternalSecretExtractsTargetName(t *testing.T) {
	externalSecret := newExternalSecretObject("payments", "payments-db", "payments-db-secret")
	externalSecret.SetUID("externalsecret-uid")
	externalSecret.SetAnnotations(map[string]string{
		AnnotationDiscoveryMode: "infer",
		AnnotationOwnerTeam:     "platform",
		AnnotationCriticality:   "high",
	})
	externalSecret.SetLabels(map[string]string{
		"app.kubernetes.io/name": "payments-api",
		"environment":            "prod",
	})

	input := WorkloadFromExternalSecret(DefaultOptions("test-cluster"), externalSecret)

	if input.Ref.Cluster != "test-cluster" ||
		input.Ref.Namespace != "payments" ||
		input.Ref.APIVersion != ExternalSecretsExternalSecretAPIVersion ||
		input.Ref.Kind != ExternalSecretsExternalSecretKind ||
		input.Ref.Name != "payments-db" ||
		input.Ref.UID != "externalsecret-uid" {
		t.Fatalf("unexpected ExternalSecret ref: %#v", input.Ref)
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

	if len(input.Edges) != 1 {
		t.Fatalf("expected one ExternalSecret edge, got %#v", input.Edges)
	}

	edge := input.Edges[0]
	if edge.Secret.Cluster != "test-cluster" ||
		edge.Secret.Namespace != "payments" ||
		edge.Secret.Name != "payments-db-secret" {
		t.Fatalf("unexpected ExternalSecret target Secret edge: %#v", edge.Secret)
	}

	if edge.DiscoveryMode != graph.DiscoveryModeInfer {
		t.Fatalf("expected inferred ExternalSecret edge, got %q", edge.DiscoveryMode)
	}

	if edge.Optional {
		t.Fatalf("expected ExternalSecret target Secret edge to be non-optional")
	}

	if len(edge.Sources) != 1 {
		t.Fatalf("expected one source, got %#v", edge.Sources)
	}

	source := edge.Sources[0]
	if source.Type != SourceExternalSecretsExternalSecretTargetName {
		t.Fatalf("expected ExternalSecret source type %q, got %q", SourceExternalSecretsExternalSecretTargetName, source.Type)
	}

	if source.Path != "spec.target.name" {
		t.Fatalf("expected source path spec.target.name, got %q", source.Path)
	}

	if source.ResourceField != "spec.target.name" {
		t.Fatalf("expected resource field spec.target.name, got %q", source.ResourceField)
	}
}

func TestWorkloadFromExternalSecretDefaultsTargetNameToMetadataName(t *testing.T) {
	externalSecret := newExternalSecretObject("payments", "payments-db", "")

	input := WorkloadFromExternalSecret(DefaultOptions("test-cluster"), externalSecret)

	if len(input.Edges) != 1 {
		t.Fatalf("expected one defaulted ExternalSecret edge, got %#v", input.Edges)
	}

	edge := input.Edges[0]
	if edge.Secret.Namespace != "payments" || edge.Secret.Name != "payments-db" {
		t.Fatalf("unexpected default ExternalSecret target Secret edge: %#v", edge.Secret)
	}

	if len(edge.Sources) != 1 || edge.Sources[0].Path != "metadata.name" {
		t.Fatalf("expected metadata.name source for defaulted target Secret, got %#v", edge.Sources)
	}
}

func TestWorkloadFromExternalSecretMergesExplicitAndInferredEdges(t *testing.T) {
	externalSecret := newExternalSecretObject("payments", "payments-db", "payments-db-secret")
	externalSecret.SetAnnotations(map[string]string{
		AnnotationDiscoveryMode: "hybrid",
		AnnotationWatchSecrets:  "payments-db-secret,shared/provider-token",
	})

	input := WorkloadFromExternalSecret(DefaultOptions("test-cluster"), externalSecret)

	edges := map[string]graph.DependencyEdge{}
	for _, edge := range input.Edges {
		edges[graph.SecretID(edge.Secret)] = edge
	}

	targetEdge, ok := edges["payments/payments-db-secret"]
	if !ok {
		t.Fatalf("expected target Secret edge, got %#v", input.Edges)
	}

	if targetEdge.DiscoveryMode != graph.DiscoveryModeHybrid {
		t.Fatalf("expected target edge mode hybrid after infer+explicit merge, got %q", targetEdge.DiscoveryMode)
	}

	if len(targetEdge.Sources) != 2 {
		t.Fatalf("expected merged target edge sources, got %#v", targetEdge.Sources)
	}

	if _, ok := edges["shared/provider-token"]; !ok {
		t.Fatalf("expected explicit cross-namespace provider token edge, got %#v", input.Edges)
	}
}

func TestWorkloadFromExternalSecretHonorsDisabledMode(t *testing.T) {
	externalSecret := newExternalSecretObject("payments", "payments-db", "payments-db-secret")
	externalSecret.SetAnnotations(map[string]string{
		AnnotationDiscoveryMode: "disabled",
		AnnotationWatchSecrets:  "shared/provider-token",
	})

	input := WorkloadFromExternalSecret(DefaultOptions("test-cluster"), externalSecret)

	if input.DiscoveryMode != graph.DiscoveryModeDisabled {
		t.Fatalf("expected disabled mode, got %q", input.DiscoveryMode)
	}

	if len(input.Edges) != 0 {
		t.Fatalf("expected no edges in disabled mode, got %#v", input.Edges)
	}
}

func TestWorkloadFromExternalSecretWithoutNameKeepsExplicitEdges(t *testing.T) {
	externalSecret := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": ExternalSecretsExternalSecretAPIVersion,
			"kind":       ExternalSecretsExternalSecretKind,
			"metadata": map[string]any{
				"namespace": "payments",
				"annotations": map[string]any{
					AnnotationDiscoveryMode: "hybrid",
					AnnotationWatchSecrets:  "shared/provider-token",
				},
			},
			"spec": map[string]any{},
		},
	}

	input := WorkloadFromExternalSecret(DefaultOptions("test-cluster"), externalSecret)

	if len(input.Edges) != 1 {
		t.Fatalf("expected one explicit edge, got %#v", input.Edges)
	}

	if input.Edges[0].Secret.Namespace != "shared" || input.Edges[0].Secret.Name != "provider-token" {
		t.Fatalf("unexpected explicit edge: %#v", input.Edges[0])
	}
}

func newExternalSecretObject(namespace, name, targetName string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": ExternalSecretsExternalSecretAPIVersion,
			"kind":       ExternalSecretsExternalSecretKind,
			"metadata": map[string]any{
				"namespace": namespace,
				"name":      name,
			},
			"spec": map[string]any{},
		},
	}

	if targetName != "" {
		_ = unstructured.SetNestedField(obj.Object, targetName, "spec", "target", "name")
	}

	return obj
}

func TestWorkloadFromExternalSecretFallbackSourceUsesMetadataName(t *testing.T) {
	externalSecret := newExternalSecretObject("payments", "payments-sync", "")

	input := WorkloadFromExternalSecret(DefaultOptions("test-cluster"), externalSecret)

	if len(input.Edges) != 1 {
		t.Fatalf("expected one fallback ExternalSecret edge, got %#v", input.Edges)
	}

	source := input.Edges[0].Sources[0]
	if source.Type != SourceExternalSecretsExternalSecretTargetName {
		t.Fatalf("expected ExternalSecret source type %q, got %q", SourceExternalSecretsExternalSecretTargetName, source.Type)
	}

	if source.Path != "metadata.name" {
		t.Fatalf("expected fallback source path metadata.name, got %q", source.Path)
	}

	if source.ResourceField != "metadata.name" {
		t.Fatalf("expected fallback source resource field metadata.name, got %q", source.ResourceField)
	}
}
