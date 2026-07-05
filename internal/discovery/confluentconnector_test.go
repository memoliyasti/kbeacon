package discovery

import (
	"testing"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWorkloadFromConfluentConnectorExtractsAuthenticationAndMountedSecrets(t *testing.T) {
	connector := newConfluentConnectorObject("payments", "jdbc-sink", map[string]any{
		"connection.password": "${file:/mnt/secrets/jdbc-credentials/password.txt:password}",
		"sasl.jaas.config":    "username=\"${file:/mnt/secrets/kafka-auth/username:username}\" password=\"${file:/mnt/secrets/kafka-auth/password:password}\"",
	})
	_ = unstructured.SetNestedMap(connector.Object, map[string]any{
		"type": "basic",
		"basic": map[string]any{
			"secretRef": map[string]any{
				"name":      "connect-rest-auth",
				"namespace": "platform",
			},
		},
	}, "spec", "connectRest", "authentication")

	input := WorkloadFromConfluentConnector(DefaultOptions("test-cluster"), connector)

	if input.Ref.Cluster != "test-cluster" ||
		input.Ref.Namespace != "payments" ||
		input.Ref.APIVersion != ConfluentConnectorAPIVersion ||
		input.Ref.Kind != ConfluentConnectorKind ||
		input.Ref.Name != "jdbc-sink" {
		t.Fatalf("unexpected Connector ref: %#v", input.Ref)
	}

	if input.DiscoveryMode != graph.DiscoveryModeHybrid {
		t.Fatalf("expected hybrid discovery mode, got %q", input.DiscoveryMode)
	}

	if len(input.Edges) != 3 {
		t.Fatalf("expected three Connector edges, got %#v", input.Edges)
	}

	assertConfluentConnectorEdge(t, input.Edges, "platform", "connect-rest-auth", SourceConfluentConnectorConnectRestAuthenticationSecretRef, "spec.connectRest.authentication.basic.secretRef")
	assertConfluentConnectorEdge(t, input.Edges, "payments", "jdbc-credentials", SourceConfluentConnectorConfigsFileMountedSecret, "spec.configs[\"connection.password\"]")
	assertConfluentConnectorEdge(t, input.Edges, "payments", "kafka-auth", SourceConfluentConnectorConfigsFileMountedSecret, "spec.configs[\"sasl.jaas.config\"]")
}

func TestWorkloadFromConfluentConnectorExtractsStringAuthenticationSecretRef(t *testing.T) {
	connector := newConfluentConnectorObject("payments", "jdbc-sink", nil)
	_ = unstructured.SetNestedField(connector.Object, "connect-rest-auth", "spec", "connectRest", "authentication", "secretRef")

	input := WorkloadFromConfluentConnector(DefaultOptions("test-cluster"), connector)

	if len(input.Edges) != 1 {
		t.Fatalf("expected one Connector auth edge, got %#v", input.Edges)
	}

	assertConfluentConnectorEdge(t, input.Edges, "payments", "connect-rest-auth", SourceConfluentConnectorConnectRestAuthenticationSecretRef, "spec.connectRest.authentication.secretRef")
}

func TestWorkloadFromConfluentConnectorMergesExplicitAndInferredEdges(t *testing.T) {
	connector := newConfluentConnectorObject("payments", "jdbc-sink", map[string]any{
		"connection.password": "${file:/mnt/secrets/jdbc-credentials/password.txt:password}",
	})
	connector.SetAnnotations(map[string]string{
		AnnotationWatchSecrets: "jdbc-credentials",
	})

	input := WorkloadFromConfluentConnector(DefaultOptions("test-cluster"), connector)

	if len(input.Edges) != 1 {
		t.Fatalf("expected one merged Connector edge, got %#v", input.Edges)
	}

	edge := input.Edges[0]
	if edge.DiscoveryMode != graph.DiscoveryModeHybrid {
		t.Fatalf("expected merged edge discovery mode hybrid, got %q", edge.DiscoveryMode)
	}

	if len(edge.Sources) != 2 {
		t.Fatalf("expected inferred and explicit sources, got %#v", edge.Sources)
	}

	types := map[string]bool{}
	for _, source := range edge.Sources {
		types[source.Type] = true
	}

	if !types[SourceConfluentConnectorConfigsFileMountedSecret] || !types["annotation"] {
		t.Fatalf("expected inferred and annotation sources, got %#v", edge.Sources)
	}
}

func TestWorkloadFromConfluentConnectorHonorsDisabledMode(t *testing.T) {
	connector := newConfluentConnectorObject("payments", "jdbc-sink", map[string]any{
		"connection.password": "${file:/mnt/secrets/jdbc-credentials/password.txt:password}",
	})
	connector.SetAnnotations(map[string]string{
		AnnotationDiscoveryMode: "disabled",
	})

	input := WorkloadFromConfluentConnector(DefaultOptions("test-cluster"), connector)

	if input.DiscoveryMode != graph.DiscoveryModeDisabled {
		t.Fatalf("expected disabled discovery mode, got %q", input.DiscoveryMode)
	}

	if len(input.Edges) != 0 {
		t.Fatalf("expected no disabled Connector edges, got %#v", input.Edges)
	}
}

func TestWorkloadFromConfluentConnectorWithoutConfigsKeepsExplicitEdges(t *testing.T) {
	connector := newConfluentConnectorObject("payments", "jdbc-sink", nil)
	connector.SetAnnotations(map[string]string{
		AnnotationWatchSecrets: "explicit-secret",
	})

	input := WorkloadFromConfluentConnector(DefaultOptions("test-cluster"), connector)

	if len(input.Edges) != 1 {
		t.Fatalf("expected one explicit Connector edge, got %#v", input.Edges)
	}

	if input.Edges[0].Secret.Namespace != "payments" || input.Edges[0].Secret.Name != "explicit-secret" {
		t.Fatalf("unexpected explicit edge: %#v", input.Edges[0])
	}

	if input.Edges[0].Sources[0].Type != "annotation" {
		t.Fatalf("expected annotation source, got %#v", input.Edges[0].Sources)
	}
}

func TestWorkloadFromConfluentConnectorIgnoresUnsupportedFileReferences(t *testing.T) {
	connector := newConfluentConnectorObject("payments", "jdbc-sink", map[string]any{
		"blank":       "",
		"configmap":   "${file:/mnt/configs/jdbc/password.txt:password}",
		"not-a-mount": "${file:/tmp/jdbc/password.txt:password}",
		"plain":       "no mounted secret reference here",
	})

	input := WorkloadFromConfluentConnector(DefaultOptions("test-cluster"), connector)

	if len(input.Edges) != 0 {
		t.Fatalf("expected no valid Connector edges, got %#v", input.Edges)
	}
}

func newConfluentConnectorObject(namespace, name string, configs map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": ConfluentConnectorAPIVersion,
			"kind":       ConfluentConnectorKind,
			"metadata": map[string]any{
				"namespace": namespace,
				"name":      name,
				"uid":       "confluent-connector-uid",
			},
			"spec": map[string]any{
				"class": "io.confluent.connect.jdbc.JdbcSinkConnector",
			},
		},
	}

	if len(configs) > 0 {
		_ = unstructured.SetNestedMap(obj.Object, configs, "spec", "configs")
	}

	return obj
}

func assertConfluentConnectorEdge(t *testing.T, edges []graph.DependencyEdge, namespace, name, sourceType, expectedPath string) {
	t.Helper()

	for _, edge := range edges {
		if edge.Secret.Namespace != namespace || edge.Secret.Name != name {
			continue
		}

		if edge.Secret.Cluster != "test-cluster" {
			t.Fatalf("unexpected Connector target cluster: %#v", edge.Secret)
		}

		if edge.DiscoveryMode != graph.DiscoveryModeInfer {
			t.Fatalf("expected inferred Connector edge, got %q", edge.DiscoveryMode)
		}

		if edge.Optional {
			t.Fatalf("expected Connector edge to be non-optional")
		}

		if len(edge.Sources) < 1 {
			t.Fatalf("expected at least one source, got %#v", edge.Sources)
		}

		for _, source := range edge.Sources {
			if source.Type != sourceType {
				continue
			}

			if source.Path != expectedPath {
				t.Fatalf("expected source path %s, got %q", expectedPath, source.Path)
			}

			if source.ResourceField != expectedPath {
				t.Fatalf("expected resource field %s, got %q", expectedPath, source.ResourceField)
			}

			return
		}

		t.Fatalf("missing source type %s on Connector edge %#v", sourceType, edge)
	}

	t.Fatalf("missing Connector edge for %s/%s in %#v", namespace, name, edges)
}
