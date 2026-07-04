package discovery

import (
	"strings"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	CertManagerCertificateAPIVersion = "cert-manager.io/v1"
	CertManagerCertificateKind       = "Certificate"

	SourceCertManagerCertificateSecretName = "cert-manager.certificate.spec.secretName"
)

// WorkloadFromCertificate normalizes a cert-manager Certificate as a
// Secret-related Kubernetes object.
//
// cert-manager Certificate is a Secret producer rather than an application
// workload, but KBeacon's current graph model represents Secret-related
// Kubernetes objects through WorkloadInput. Runtime docs should continue to
// describe this carefully when the watcher is wired.
func WorkloadFromCertificate(opts Options, certificate *unstructured.Unstructured) graph.WorkloadInput {
	if certificate == nil {
		return graph.WorkloadInput{}
	}

	annotations := certificate.GetAnnotations()
	labels := certificate.GetLabels()
	labelKeys := opts.MetadataLabelKeys.withDefaults()
	mode := discoveryModeFor(annotations, opts.DefaultMode)

	apiVersion := strings.TrimSpace(certificate.GetAPIVersion())
	if apiVersion == "" {
		apiVersion = CertManagerCertificateAPIVersion
	}

	kind := strings.TrimSpace(certificate.GetKind())
	if kind == "" {
		kind = CertManagerCertificateKind
	}

	ref := graph.WorkloadRef{
		Cluster:    opts.Cluster,
		Namespace:  certificate.GetNamespace(),
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       certificate.GetName(),
		UID:        string(certificate.GetUID()),
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

	ignore := ignoredSecrets(opts.Cluster, certificate.GetNamespace(), annotations)
	edges := []graph.DependencyEdge{}

	if mode == graph.DiscoveryModeInfer || mode == graph.DiscoveryModeHybrid {
		if secretName, ok := certificateSecretName(certificate); ok {
			edges = append(edges, newEdge(
				opts.Cluster,
				ref,
				ref.Namespace,
				secretName,
				false,
				graph.DiscoveryModeInfer,
				graph.DependencySource{
					Type:          SourceCertManagerCertificateSecretName,
					Path:          "spec.secretName",
					ResourceField: "spec.secretName",
				},
			))
		}
	}

	if mode == graph.DiscoveryModeExplicit || mode == graph.DiscoveryModeHybrid {
		edges = append(edges, explicitAnnotationEdges(opts.Cluster, ref, annotations)...)
	}

	input.Edges = filterAndMergeEdges(edges, ignore)
	return input
}

func certificateSecretName(certificate *unstructured.Unstructured) (string, bool) {
	secretName, found, err := unstructured.NestedString(certificate.Object, "spec", "secretName")
	if err != nil || !found {
		return "", false
	}

	secretName = strings.TrimSpace(secretName)
	if secretName == "" {
		return "", false
	}

	return secretName, true
}
