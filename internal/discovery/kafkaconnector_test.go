package discovery

import (
	"testing"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWorkloadFromStrimziKafkaConnectorExtractsConfigProviderSecrets(t *testing.T) {
	kafkaConnector := newStrimziKafkaConnectorObject("payments", "mysql-source", map[string]any{
		"database.password": "${secrets:payments/mysql-connector-auth:password}",
		"sasl.jaas.config":  "username=\"${secrets:shared/kafka-auth:username}\" password=\"${secrets:kafka-auth:password}\"",
		"tasks.max":         int64(1),
	})

	input := WorkloadFromStrimziKafkaConnector(DefaultOptions("test-cluster"), kafkaConnector)

	if input.Ref.Cluster != "test-cluster" ||
		input.Ref.Namespace != "payments" ||
		input.Ref.APIVersion != StrimziKafkaConnectorAPIVersion ||
		input.Ref.Kind != StrimziKafkaConnectorKind ||
		input.Ref.Name != "mysql-source" {
		t.Fatalf("unexpected KafkaConnector ref: %#v", input.Ref)
	}

	if input.DiscoveryMode != graph.DiscoveryModeHybrid {
		t.Fatalf("expected hybrid discovery mode, got %q", input.DiscoveryMode)
	}

	if len(input.Edges) != 3 {
		t.Fatalf("expected three KafkaConnector edges, got %#v", input.Edges)
	}

	assertStrimziKafkaConnectorEdge(t, input.Edges, "payments", "mysql-connector-auth", "spec.config[\"database.password\"]")
	assertStrimziKafkaConnectorEdge(t, input.Edges, "shared", "kafka-auth", "spec.config[\"sasl.jaas.config\"]")
	assertStrimziKafkaConnectorEdge(t, input.Edges, "payments", "kafka-auth", "spec.config[\"sasl.jaas.config\"]")
}

func TestWorkloadFromStrimziKafkaConnectorMergesExplicitAndInferredEdges(t *testing.T) {
	kafkaConnector := newStrimziKafkaConnectorObject("payments", "mysql-source", map[string]any{
		"database.password": "${secrets:payments-db:password}",
	})
	kafkaConnector.SetAnnotations(map[string]string{
		AnnotationWatchSecrets: "payments-db",
	})

	input := WorkloadFromStrimziKafkaConnector(DefaultOptions("test-cluster"), kafkaConnector)

	if len(input.Edges) != 1 {
		t.Fatalf("expected one merged KafkaConnector edge, got %#v", input.Edges)
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

	if !types[SourceStrimziKafkaConnectorConfigProviderSecrets] || !types["annotation"] {
		t.Fatalf("expected inferred and annotation sources, got %#v", edge.Sources)
	}
}

func TestWorkloadFromStrimziKafkaConnectorHonorsDisabledMode(t *testing.T) {
	kafkaConnector := newStrimziKafkaConnectorObject("payments", "mysql-source", map[string]any{
		"database.password": "${secrets:payments-db:password}",
	})
	kafkaConnector.SetAnnotations(map[string]string{
		AnnotationDiscoveryMode: "disabled",
	})

	input := WorkloadFromStrimziKafkaConnector(DefaultOptions("test-cluster"), kafkaConnector)

	if input.DiscoveryMode != graph.DiscoveryModeDisabled {
		t.Fatalf("expected disabled discovery mode, got %q", input.DiscoveryMode)
	}

	if len(input.Edges) != 0 {
		t.Fatalf("expected no disabled KafkaConnector edges, got %#v", input.Edges)
	}
}

func TestWorkloadFromStrimziKafkaConnectorWithoutConfigKeepsExplicitEdges(t *testing.T) {
	kafkaConnector := newStrimziKafkaConnectorObject("payments", "mysql-source", nil)
	kafkaConnector.SetAnnotations(map[string]string{
		AnnotationWatchSecrets: "explicit-secret",
	})

	input := WorkloadFromStrimziKafkaConnector(DefaultOptions("test-cluster"), kafkaConnector)

	if len(input.Edges) != 1 {
		t.Fatalf("expected one explicit KafkaConnector edge, got %#v", input.Edges)
	}

	if input.Edges[0].Secret.Namespace != "payments" || input.Edges[0].Secret.Name != "explicit-secret" {
		t.Fatalf("unexpected explicit edge: %#v", input.Edges[0])
	}

	if input.Edges[0].Sources[0].Type != "annotation" {
		t.Fatalf("expected annotation source, got %#v", input.Edges[0].Sources)
	}
}

func TestWorkloadFromStrimziKafkaConnectorIgnoresInvalidConfigProviderReferences(t *testing.T) {
	kafkaConnector := newStrimziKafkaConnectorObject("payments", "mysql-source", map[string]any{
		"empty":     "",
		"missing":   "${secrets::password}",
		"bad-slash": "${secrets:namespace/secret/extra:password}",
		"plain":     "no secret reference here",
	})

	input := WorkloadFromStrimziKafkaConnector(DefaultOptions("test-cluster"), kafkaConnector)

	if len(input.Edges) != 0 {
		t.Fatalf("expected no valid KafkaConnector edges, got %#v", input.Edges)
	}
}

func newStrimziKafkaConnectorObject(namespace, name string, config map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": StrimziKafkaConnectorAPIVersion,
			"kind":       StrimziKafkaConnectorKind,
			"metadata": map[string]any{
				"namespace": namespace,
				"name":      name,
				"uid":       "strimzi-kafkaconnector-uid",
			},
			"spec": map[string]any{
				"class": "io.debezium.connector.mysql.MySqlConnector",
			},
		},
	}

	if len(config) > 0 {
		_ = unstructured.SetNestedMap(obj.Object, config, "spec", "config")
	}

	return obj
}

func assertStrimziKafkaConnectorEdge(t *testing.T, edges []graph.DependencyEdge, namespace, name, expectedPath string) {
	t.Helper()

	for _, edge := range edges {
		if edge.Secret.Namespace != namespace || edge.Secret.Name != name {
			continue
		}

		if edge.Secret.Cluster != "test-cluster" {
			t.Fatalf("unexpected KafkaConnector target cluster: %#v", edge.Secret)
		}

		if edge.DiscoveryMode != graph.DiscoveryModeInfer {
			t.Fatalf("expected inferred KafkaConnector edge, got %q", edge.DiscoveryMode)
		}

		if edge.Optional {
			t.Fatalf("expected KafkaConnector edge to be non-optional")
		}

		if len(edge.Sources) != 1 {
			t.Fatalf("expected one source, got %#v", edge.Sources)
		}

		source := edge.Sources[0]
		if source.Type != SourceStrimziKafkaConnectorConfigProviderSecrets {
			t.Fatalf("expected KafkaConnector source type %q, got %q", SourceStrimziKafkaConnectorConfigProviderSecrets, source.Type)
		}

		if source.Path != expectedPath {
			t.Fatalf("expected source path %s, got %q", expectedPath, source.Path)
		}

		if source.ResourceField != expectedPath {
			t.Fatalf("expected resource field %s, got %q", expectedPath, source.ResourceField)
		}

		return
	}

	t.Fatalf("missing KafkaConnector edge for %s/%s in %#v", namespace, name, edges)
}
