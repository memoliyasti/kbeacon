package discovery

import (
	"strings"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ExternalSecretsExternalSecretAPIVersion = "external-secrets.io/v1"
	ExternalSecretsExternalSecretKind       = "ExternalSecret"

	SourceExternalSecretsExternalSecretTargetName = "external-secrets.externalsecret.spec.target.name"
)

// WorkloadFromExternalSecret normalizes an External Secrets Operator
// ExternalSecret as a Secret-related Kubernetes object.
//
// ExternalSecret is a Secret producer/synchronizer rather than an
// application workload, but KBeacon's current graph model represents
// Secret-related Kubernetes objects through WorkloadInput.
func WorkloadFromExternalSecret(opts Options, externalSecret *unstructured.Unstructured) graph.WorkloadInput {
	if externalSecret == nil {
		return graph.WorkloadInput{}
	}

	annotations := externalSecret.GetAnnotations()
	labels := externalSecret.GetLabels()
	labelKeys := opts.MetadataLabelKeys.withDefaults()
	mode := discoveryModeFor(annotations, opts.DefaultMode)

	apiVersion := strings.TrimSpace(externalSecret.GetAPIVersion())
	if apiVersion == "" {
		apiVersion = ExternalSecretsExternalSecretAPIVersion
	}

	kind := strings.TrimSpace(externalSecret.GetKind())
	if kind == "" {
		kind = ExternalSecretsExternalSecretKind
	}

	ref := graph.WorkloadRef{
		Cluster:    opts.Cluster,
		Namespace:  externalSecret.GetNamespace(),
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       externalSecret.GetName(),
		UID:        string(externalSecret.GetUID()),
	}

	input := graph.WorkloadInput{
		Ref:           ref,
		OwnerTeam:     metadataValue(annotations[AnnotationOwnerTeam], labels, labelKeys.OwnerTeam, opts.MetadataLabelsEnabled),
		Service:       metadataValue(annotations[AnnotationService], labels, labelKeys.Service, opts.MetadataLabelsEnabled),
		Environment:   metadataValue(annotations[AnnotationEnvironment], labels, labelKeys.Environment, opts.MetadataLabelsEnabled),
		Criticality:   metadataValue(annotations[AnnotationCriticality], labels, labelKeys.Criticality, opts.MetadataLabelsEnabled),
		DiscoveryMode: mode,
	}

	if mode == graph.DiscoveryModeDisabled {
		return input
	}

	ignore := ignoredSecrets(opts.Cluster, externalSecret.GetNamespace(), annotations)
	edges := []graph.DependencyEdge{}

	if mode == graph.DiscoveryModeInfer || mode == graph.DiscoveryModeHybrid {
		if secretName, source, ok := externalSecretTargetSecretName(externalSecret); ok {
			edges = append(edges, newEdge(
				opts.Cluster,
				ref,
				ref.Namespace,
				secretName,
				false,
				graph.DiscoveryModeInfer,
				source,
			))
		}
	}

	if mode == graph.DiscoveryModeExplicit || mode == graph.DiscoveryModeHybrid {
		edges = append(edges, explicitAnnotationEdges(opts.Cluster, ref, annotations)...)
	}

	input.Edges = filterAndMergeEdges(edges, ignore)
	return input
}

func externalSecretTargetSecretName(externalSecret *unstructured.Unstructured) (string, graph.DependencySource, bool) {
	source := graph.DependencySource{
		Type:          SourceExternalSecretsExternalSecretTargetName,
		Path:          "spec.target.name",
		ResourceField: "spec.target.name",
	}

	secretName, found, err := unstructured.NestedString(externalSecret.Object, "spec", "target", "name")
	if err == nil && found {
		secretName = strings.TrimSpace(secretName)
		if secretName != "" {
			return secretName, source, true
		}
	}

	secretName = strings.TrimSpace(externalSecret.GetName())
	if secretName == "" {
		return "", graph.DependencySource{}, false
	}

	source.Path = "metadata.name"
	source.ResourceField = "metadata.name"
	return secretName, source, true
}
