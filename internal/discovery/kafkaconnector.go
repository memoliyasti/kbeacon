package discovery

import (
	"regexp"
	"strings"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	StrimziKafkaConnectorAPIVersion = "kafka.strimzi.io/v1beta2"
	StrimziKafkaConnectorKind       = "KafkaConnector"

	SourceStrimziKafkaConnectorConfigProviderSecrets = "strimzi.kafkaconnector.spec.config.secrets"
)

var strimziKafkaConnectorSecretRefPattern = regexp.MustCompile(`\$\{secrets:([^}:]+):([^}]+)\}`)

// WorkloadFromStrimziKafkaConnector normalizes a Strimzi KafkaConnector as a
// Secret-related Kubernetes object.
//
// Strimzi KafkaConnector config values can use the Strimzi Kubernetes
// Configuration Provider syntax, for example:
//
//	${secrets:namespace/secret:key}
//	${secrets:secret:key}
//
// KBeacon models only the referenced Kubernetes Secret name and namespace. It
// does not inspect connector credentials, provider payloads, mounted files, or
// Secret values.
func WorkloadFromStrimziKafkaConnector(opts Options, kafkaConnector *unstructured.Unstructured) graph.WorkloadInput {
	annotations := kafkaConnector.GetAnnotations()
	mode := discoveryModeFor(annotations, opts.DefaultMode)

	apiVersion := kafkaConnector.GetAPIVersion()
	if strings.TrimSpace(apiVersion) == "" {
		apiVersion = StrimziKafkaConnectorAPIVersion
	}

	kind := kafkaConnector.GetKind()
	if strings.TrimSpace(kind) == "" {
		kind = StrimziKafkaConnectorKind
	}

	ref := graph.WorkloadRef{
		Cluster:    opts.Cluster,
		Namespace:  kafkaConnector.GetNamespace(),
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       kafkaConnector.GetName(),
		UID:        string(kafkaConnector.GetUID()),
	}

	input := graph.WorkloadInput{
		Ref:           ref,
		OwnerTeam:     annotations[AnnotationOwnerTeam],
		Service:       annotations[AnnotationService],
		Environment:   annotations[AnnotationEnvironment],
		Criticality:   annotations[AnnotationCriticality],
		DiscoveryMode: mode,
	}

	if mode == graph.DiscoveryModeDisabled {
		return input
	}

	ignore := ignoredSecrets(opts.Cluster, ref.Namespace, annotations)
	edges := []graph.DependencyEdge{}

	if mode == graph.DiscoveryModeInfer || mode == graph.DiscoveryModeHybrid {
		edges = append(edges, inferStrimziKafkaConnectorEdges(opts, ref, kafkaConnector)...)
	}

	if mode == graph.DiscoveryModeExplicit || mode == graph.DiscoveryModeHybrid {
		edges = append(edges, explicitAnnotationEdges(opts.Cluster, ref, annotations)...)
	}

	input.Edges = filterAndMergeEdges(edges, ignore)
	return input
}

func inferStrimziKafkaConnectorEdges(opts Options, workload graph.WorkloadRef, kafkaConnector *unstructured.Unstructured) []graph.DependencyEdge {
	config, found, err := unstructured.NestedMap(kafkaConnector.Object, "spec", "config")
	if err != nil || !found {
		return nil
	}

	edges := []graph.DependencyEdge{}

	for _, key := range crdSortedKeys(config) {
		value, ok := config[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}

		path := crdMapFieldPath("spec.config", key)
		matches := strimziKafkaConnectorSecretRefPattern.FindAllStringSubmatch(value, -1)

		for _, match := range matches {
			if len(match) != 3 {
				continue
			}

			secretNamespace, secretName := parseStrimziKafkaConnectorSecretTarget(workload.Namespace, match[1])
			if secretNamespace == "" || secretName == "" {
				continue
			}

			edges = append(edges, newEdge(
				opts.Cluster,
				workload,
				secretNamespace,
				secretName,
				false,
				graph.DiscoveryModeInfer,
				graph.DependencySource{
					Type:          SourceStrimziKafkaConnectorConfigProviderSecrets,
					Path:          path,
					ResourceField: path,
				},
			))
		}
	}

	return edges
}

func parseStrimziKafkaConnectorSecretTarget(defaultNamespace, token string) (string, string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ""
	}

	namespace := defaultNamespace
	name := token

	if slash := strings.Index(token, "/"); slash >= 0 {
		namespace = strings.TrimSpace(token[:slash])
		name = strings.TrimSpace(token[slash+1:])
	}

	if namespace == "" || name == "" || strings.Contains(name, "/") {
		return "", ""
	}

	return namespace, name
}
