package discovery

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ConfluentConnectorAPIVersion = "platform.confluent.io/v1beta1"
	ConfluentConnectorKind       = "Connector"

	SourceConfluentConnectorConnectRestAuthenticationSecretRef = "confluent.connector.spec.connectRest.authentication.secretRef"
	SourceConfluentConnectorConfigsFileMountedSecret           = "confluent.connector.spec.configs.file.mountedSecret"
)

var confluentConnectorFileRefPattern = regexp.MustCompile(`\$\{file:([^}:]+):([^}]+)\}`)

// WorkloadFromConfluentConnector normalizes a Confluent for Kubernetes
// Connector as a Secret-related Kubernetes object.
//
// KBeacon models only Kubernetes Secret references visible in the Connector
// CRD:
//   - spec.connectRest.authentication.*.secretRef
//   - spec.configs string values that use mounted Secret file references such
//     as ${file:/mnt/secrets/<secret>/...:key}
//
// KBeacon does not call Kafka Connect REST APIs and does not inspect mounted
// file contents, connector credentials, or Secret values.
func WorkloadFromConfluentConnector(opts Options, connector *unstructured.Unstructured) graph.WorkloadInput {
	annotations := connector.GetAnnotations()
	mode := discoveryModeFor(annotations, opts.DefaultMode)

	apiVersion := connector.GetAPIVersion()
	if strings.TrimSpace(apiVersion) == "" {
		apiVersion = ConfluentConnectorAPIVersion
	}

	kind := connector.GetKind()
	if strings.TrimSpace(kind) == "" {
		kind = ConfluentConnectorKind
	}

	ref := graph.WorkloadRef{
		Cluster:    opts.Cluster,
		Namespace:  connector.GetNamespace(),
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       connector.GetName(),
		UID:        string(connector.GetUID()),
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
		edges = append(edges, inferConfluentConnectorEdges(opts, ref, connector)...)
	}

	if mode == graph.DiscoveryModeExplicit || mode == graph.DiscoveryModeHybrid {
		edges = append(edges, explicitAnnotationEdges(opts.Cluster, ref, annotations)...)
	}

	input.Edges = filterAndMergeEdges(edges, ignore)
	return input
}

func inferConfluentConnectorEdges(opts Options, workload graph.WorkloadRef, connector *unstructured.Unstructured) []graph.DependencyEdge {
	edges := []graph.DependencyEdge{}
	edges = append(edges, inferConfluentConnectorConnectRestAuthenticationEdges(opts, workload, connector)...)
	edges = append(edges, inferConfluentConnectorMountedSecretEdges(opts, workload, connector)...)
	return edges
}

func inferConfluentConnectorConnectRestAuthenticationEdges(opts Options, workload graph.WorkloadRef, connector *unstructured.Unstructured) []graph.DependencyEdge {
	authentication, found, err := unstructured.NestedMap(connector.Object, "spec", "connectRest", "authentication")
	if err != nil || !found {
		return nil
	}

	return scanConfluentConnectorSecretRefs(opts, workload, authentication, []string{"spec", "connectRest", "authentication"})
}

func scanConfluentConnectorSecretRefs(opts Options, workload graph.WorkloadRef, value any, path []string) []graph.DependencyEdge {
	edges := []graph.DependencyEdge{}

	switch typed := value.(type) {
	case map[string]any:
		for _, key := range crdSortedKeys(typed) {
			childPath := appendPath(path, key)
			childValue := typed[key]

			if strings.EqualFold(key, "secretRef") {
				secretNamespace, secretName := parseConfluentConnectorSecretRef(workload.Namespace, childValue)
				if secretNamespace != "" && secretName != "" {
					sourcePath := crdJoinPath(childPath...)
					edges = append(edges, newEdge(
						opts.Cluster,
						workload,
						secretNamespace,
						secretName,
						false,
						graph.DiscoveryModeInfer,
						graph.DependencySource{
							Type:          SourceConfluentConnectorConnectRestAuthenticationSecretRef,
							Path:          sourcePath,
							ResourceField: sourcePath,
						},
					))
				}
			}

			edges = append(edges, scanConfluentConnectorSecretRefs(opts, workload, childValue, childPath)...)
		}

	case []any:
		for i, item := range typed {
			childPath := appendPath(path, fmt.Sprintf("[%d]", i))
			edges = append(edges, scanConfluentConnectorSecretRefs(opts, workload, item, childPath)...)
		}
	}

	return edges
}

func parseConfluentConnectorSecretRef(defaultNamespace string, value any) (string, string) {
	switch typed := value.(type) {
	case string:
		name := strings.TrimSpace(typed)
		if name == "" {
			return "", ""
		}
		return defaultNamespace, name

	case map[string]any:
		name := firstStringField(typed, "name", "secretName")
		namespace := firstStringField(typed, "namespace")
		if namespace == "" {
			namespace = defaultNamespace
		}
		if name == "" || namespace == "" {
			return "", ""
		}
		return namespace, name

	default:
		return "", ""
	}
}

func inferConfluentConnectorMountedSecretEdges(opts Options, workload graph.WorkloadRef, connector *unstructured.Unstructured) []graph.DependencyEdge {
	configs, found, err := unstructured.NestedMap(connector.Object, "spec", "configs")
	if err != nil || !found {
		return nil
	}

	edges := []graph.DependencyEdge{}

	for _, key := range crdSortedKeys(configs) {
		value, ok := configs[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}

		path := crdMapFieldPath("spec.configs", key)
		matches := confluentConnectorFileRefPattern.FindAllStringSubmatch(value, -1)

		for _, match := range matches {
			if len(match) != 3 {
				continue
			}

			secretName := parseConfluentConnectorMountedSecretName(match[1])
			if secretName == "" {
				continue
			}

			edges = append(edges, newEdge(
				opts.Cluster,
				workload,
				workload.Namespace,
				secretName,
				false,
				graph.DiscoveryModeInfer,
				graph.DependencySource{
					Type:          SourceConfluentConnectorConfigsFileMountedSecret,
					Path:          path,
					ResourceField: path,
				},
			))
		}
	}

	return edges
}

func parseConfluentConnectorMountedSecretName(path string) string {
	path = strings.TrimSpace(path)

	const mountedSecretPrefix = "/mnt/secrets/"
	if !strings.HasPrefix(path, mountedSecretPrefix) {
		return ""
	}

	remainder := strings.TrimPrefix(path, mountedSecretPrefix)
	remainder = strings.TrimLeft(remainder, "/")
	if remainder == "" {
		return ""
	}

	secretName := remainder
	if slash := strings.Index(remainder, "/"); slash >= 0 {
		secretName = remainder[:slash]
	}

	secretName = strings.TrimSpace(secretName)
	if secretName == "" || strings.Contains(secretName, "/") {
		return ""
	}

	return secretName
}

func firstStringField(values map[string]any, names ...string) string {
	for _, name := range names {
		if value, ok := values[name].(string); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func appendPath(path []string, item string) []string {
	out := make([]string, 0, len(path)+1)
	out = append(out, path...)
	out = append(out, item)
	return out
}
