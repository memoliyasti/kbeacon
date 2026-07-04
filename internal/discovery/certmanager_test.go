package discovery

import (
	"testing"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWorkloadFromCertificateExtractsSecretName(t *testing.T) {
	certificate := newCertificateObject("payments", "payments-tls", "payments-tls-secret")
	certificate.SetUID("certificate-uid")
	certificate.SetAnnotations(map[string]string{
		AnnotationDiscoveryMode: "infer",
		AnnotationOwnerTeam:     "platform",
		AnnotationCriticality:   "high",
	})
	certificate.SetLabels(map[string]string{
		"app.kubernetes.io/name": "payments-web",
		"environment":            "prod",
	})

	input := WorkloadFromCertificate(DefaultOptions("test-cluster"), certificate)

	if input.Ref.Cluster != "test-cluster" ||
		input.Ref.Namespace != "payments" ||
		input.Ref.APIVersion != CertManagerCertificateAPIVersion ||
		input.Ref.Kind != CertManagerCertificateKind ||
		input.Ref.Name != "payments-tls" ||
		input.Ref.UID != "certificate-uid" {
		t.Fatalf("unexpected Certificate ref: %#v", input.Ref)
	}

	if input.OwnerTeam != "platform" {
		t.Fatalf("expected owner team platform, got %q", input.OwnerTeam)
	}

	if input.Criticality != "high" {
		t.Fatalf("expected criticality high, got %q", input.Criticality)
	}

	if input.Service != "payments-web" {
		t.Fatalf("expected service label fallback, got %q", input.Service)
	}

	if input.Environment != "prod" {
		t.Fatalf("expected environment label fallback, got %q", input.Environment)
	}

	if len(input.Edges) != 1 {
		t.Fatalf("expected one Certificate edge, got %#v", input.Edges)
	}

	edge := input.Edges[0]
	if edge.Secret.Cluster != "test-cluster" ||
		edge.Secret.Namespace != "payments" ||
		edge.Secret.Name != "payments-tls-secret" {
		t.Fatalf("unexpected Certificate target Secret edge: %#v", edge.Secret)
	}

	if edge.DiscoveryMode != graph.DiscoveryModeInfer {
		t.Fatalf("expected inferred Certificate edge, got %q", edge.DiscoveryMode)
	}

	if edge.Optional {
		t.Fatalf("expected Certificate target Secret edge to be non-optional")
	}

	if len(edge.Sources) != 1 {
		t.Fatalf("expected one source, got %#v", edge.Sources)
	}

	source := edge.Sources[0]
	if source.Type != SourceCertManagerCertificateSecretName {
		t.Fatalf("expected Certificate source type %q, got %q", SourceCertManagerCertificateSecretName, source.Type)
	}

	if source.Path != "spec.secretName" {
		t.Fatalf("expected source path spec.secretName, got %q", source.Path)
	}

	if source.ResourceField != "spec.secretName" {
		t.Fatalf("expected resource field spec.secretName, got %q", source.ResourceField)
	}
}

func TestWorkloadFromCertificateMergesExplicitAndInferredEdges(t *testing.T) {
	certificate := newCertificateObject("payments", "payments-tls", "payments-tls-secret")
	certificate.SetAnnotations(map[string]string{
		AnnotationDiscoveryMode: "hybrid",
		AnnotationWatchSecrets:  "payments-tls-secret,shared/issuer-account",
	})

	input := WorkloadFromCertificate(DefaultOptions("test-cluster"), certificate)

	edges := map[string]graph.DependencyEdge{}
	for _, edge := range input.Edges {
		edges[graph.SecretID(edge.Secret)] = edge
	}

	targetEdge, ok := edges["payments/payments-tls-secret"]
	if !ok {
		t.Fatalf("expected target Secret edge, got %#v", input.Edges)
	}

	if targetEdge.DiscoveryMode != graph.DiscoveryModeHybrid {
		t.Fatalf("expected target edge mode hybrid after infer+explicit merge, got %q", targetEdge.DiscoveryMode)
	}

	if len(targetEdge.Sources) != 2 {
		t.Fatalf("expected merged target edge sources, got %#v", targetEdge.Sources)
	}

	if _, ok := edges["shared/issuer-account"]; !ok {
		t.Fatalf("expected explicit cross-namespace issuer edge, got %#v", input.Edges)
	}
}

func TestWorkloadFromCertificateHonorsDisabledMode(t *testing.T) {
	certificate := newCertificateObject("payments", "payments-tls", "payments-tls-secret")
	certificate.SetAnnotations(map[string]string{
		AnnotationDiscoveryMode: "disabled",
		AnnotationWatchSecrets:  "shared/issuer-account",
	})

	input := WorkloadFromCertificate(DefaultOptions("test-cluster"), certificate)

	if input.DiscoveryMode != graph.DiscoveryModeDisabled {
		t.Fatalf("expected disabled mode, got %q", input.DiscoveryMode)
	}

	if len(input.Edges) != 0 {
		t.Fatalf("expected no edges in disabled mode, got %#v", input.Edges)
	}
}

func TestWorkloadFromCertificateWithoutSecretNameKeepsExplicitEdges(t *testing.T) {
	certificate := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": CertManagerCertificateAPIVersion,
			"kind":       CertManagerCertificateKind,
			"metadata": map[string]any{
				"namespace": "payments",
				"name":      "payments-tls",
				"annotations": map[string]any{
					AnnotationDiscoveryMode: "hybrid",
					AnnotationWatchSecrets:  "shared/issuer-account",
				},
			},
			"spec": map[string]any{},
		},
	}

	input := WorkloadFromCertificate(DefaultOptions("test-cluster"), certificate)

	if len(input.Edges) != 1 {
		t.Fatalf("expected one explicit edge, got %#v", input.Edges)
	}

	if input.Edges[0].Secret.Namespace != "shared" || input.Edges[0].Secret.Name != "issuer-account" {
		t.Fatalf("unexpected explicit edge: %#v", input.Edges[0])
	}
}
