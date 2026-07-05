package discovery

import (
	"fmt"
	"strings"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	SecretsStoreSecretProviderClassAPIVersion = "secrets-store.csi.x-k8s.io/v1"
	SecretsStoreSecretProviderClassKind       = "SecretProviderClass"

	SourceSecretsStoreSecretProviderClassSecretObjects = "secrets-store.csi.secretproviderclass.spec.secretObjects.secretName"
)

// WorkloadFromSecretProviderClass normalizes a Secrets Store CSI Driver
// SecretProviderClass as a Secret-related Kubernetes object.
//
// SecretProviderClass is commonly consumed by Pods through the
// secrets-store.csi.k8s.io CSI driver. Its spec.secretObjects entries can
// ask the driver to sync external provider material into Kubernetes Secrets.
// KBeacon models those Kubernetes Secret outputs as dependency edges.
func WorkloadFromSecretProviderClass(opts Options, secretProviderClass *unstructured.Unstructured) graph.WorkloadInput {
	if secretProviderClass == nil {
		return graph.WorkloadInput{}
	}

	annotations := secretProviderClass.GetAnnotations()
	labels := secretProviderClass.GetLabels()
	labelKeys := opts.MetadataLabelKeys.withDefaults()
	mode := discoveryModeFor(annotations, opts.DefaultMode)

	apiVersion := strings.TrimSpace(secretProviderClass.GetAPIVersion())
	if apiVersion == "" {
		apiVersion = SecretsStoreSecretProviderClassAPIVersion
	}

	kind := strings.TrimSpace(secretProviderClass.GetKind())
	if kind == "" {
		kind = SecretsStoreSecretProviderClassKind
	}

	ref := graph.WorkloadRef{
		Cluster:    opts.Cluster,
		Namespace:  secretProviderClass.GetNamespace(),
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       secretProviderClass.GetName(),
		UID:        string(secretProviderClass.GetUID()),
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

	ignore := ignoredSecrets(opts.Cluster, secretProviderClass.GetNamespace(), annotations)
	edges := []graph.DependencyEdge{}

	if mode == graph.DiscoveryModeInfer || mode == graph.DiscoveryModeHybrid {
		for _, target := range secretProviderClassSecretObjectTargets(secretProviderClass) {
			edges = append(edges, newEdge(
				opts.Cluster,
				ref,
				ref.Namespace,
				target.secretName,
				false,
				graph.DiscoveryModeInfer,
				target.source,
			))
		}
	}

	if mode == graph.DiscoveryModeExplicit || mode == graph.DiscoveryModeHybrid {
		edges = append(edges, explicitAnnotationEdges(opts.Cluster, ref, annotations)...)
	}

	input.Edges = filterAndMergeEdges(edges, ignore)
	return input
}

type secretProviderClassTarget struct {
	secretName string
	source     graph.DependencySource
}

func secretProviderClassSecretObjectTargets(secretProviderClass *unstructured.Unstructured) []secretProviderClassTarget {
	items, found, err := unstructured.NestedSlice(secretProviderClass.Object, "spec", "secretObjects")
	if err != nil || !found {
		return nil
	}

	targets := make([]secretProviderClassTarget, 0, len(items))

	for i, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}

		secretName, found, err := unstructured.NestedString(object, "secretName")
		if err != nil || !found {
			continue
		}

		secretName = strings.TrimSpace(secretName)
		if secretName == "" {
			continue
		}

		path := fmt.Sprintf("spec.secretObjects[%d].secretName", i)
		targets = append(targets, secretProviderClassTarget{
			secretName: secretName,
			source: graph.DependencySource{
				Type:          SourceSecretsStoreSecretProviderClassSecretObjects,
				Path:          path,
				ResourceField: path,
			},
		})
	}

	return targets
}
