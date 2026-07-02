package discovery

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/memoliyasti/kbeacon/internal/graph"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MetadataLabelKeyConfig struct {
	OwnerTeam   []string
	Service     []string
	Environment []string
	Criticality []string
}

func DefaultMetadataLabelKeyConfig() MetadataLabelKeyConfig {
	return MetadataLabelKeyConfig{
		OwnerTeam: []string{
			"app.kubernetes.io/team",
			"team",
			"owner-team",
			"ownerTeam",
			"technical-owner",
			"technicalOwner",
			"business-owner",
			"businessOwner",
		},
		Service: []string{
			"app.kubernetes.io/name",
			"app",
			"service",
			"service-name",
			"serviceName",
		},
		Environment: []string{
			"app.kubernetes.io/environment",
			"environment",
			"env",
			"stage",
		},
		Criticality: []string{
			"app.kubernetes.io/criticality",
			"criticality",
			"priority",
			"tier",
			"slo-tier",
		},
	}
}

type Options struct {
	Cluster                        string
	DefaultMode                    graph.DiscoveryMode
	IncludeImagePullSecrets        bool
	ServiceAccountImagePullSecrets map[string][]string
	IncludeInitContainers          bool
	IncludeEphemeralContainers     bool
	ReadPodTemplateAnnotations     bool
	MetadataLabelsEnabled          bool
	MetadataLabelKeys              MetadataLabelKeyConfig
}

func DefaultOptions(cluster string) Options {
	return Options{
		Cluster:                    cluster,
		DefaultMode:                graph.DiscoveryModeHybrid,
		IncludeImagePullSecrets:    true,
		IncludeInitContainers:      true,
		IncludeEphemeralContainers: true,
		ReadPodTemplateAnnotations: true,
		MetadataLabelsEnabled:      true,
		MetadataLabelKeys:          DefaultMetadataLabelKeyConfig(),
	}
}

func SecretInputFromSecret(cluster string, secret *corev1.Secret) graph.SecretInput {
	annotations := secret.GetAnnotations()

	return graph.SecretInput{
		Ref: graph.SecretRef{
			Cluster:   cluster,
			Namespace: secret.Namespace,
			Name:      secret.Name,
		},
		Type:              string(secret.Type),
		OwnerTeam:         annotations[AnnotationOwnerTeam],
		Criticality:       annotations[AnnotationCriticality],
		ResourceVersion:   secret.ResourceVersion,
		CreationTimestamp: secret.CreationTimestamp.Time,
	}
}

func WorkloadFromDeployment(opts Options, deployment *appsv1.Deployment) graph.WorkloadInput {
	return workloadFromPodSpec(
		opts,
		"apps/v1",
		"Deployment",
		deployment.Namespace,
		deployment.Name,
		string(deployment.UID),
		deployment.Annotations,
		deployment.Labels,
		deployment.Spec.Template.Annotations,
		deployment.Spec.Template.Labels,
		deployment.Spec.Template.Spec,
	)
}

func WorkloadFromStatefulSet(opts Options, statefulSet *appsv1.StatefulSet) graph.WorkloadInput {
	return workloadFromPodSpec(
		opts,
		"apps/v1",
		"StatefulSet",
		statefulSet.Namespace,
		statefulSet.Name,
		string(statefulSet.UID),
		statefulSet.Annotations,
		statefulSet.Labels,
		statefulSet.Spec.Template.Annotations,
		statefulSet.Spec.Template.Labels,
		statefulSet.Spec.Template.Spec,
	)
}

func WorkloadFromDaemonSet(opts Options, daemonSet *appsv1.DaemonSet) graph.WorkloadInput {
	return workloadFromPodSpec(
		opts,
		"apps/v1",
		"DaemonSet",
		daemonSet.Namespace,
		daemonSet.Name,
		string(daemonSet.UID),
		daemonSet.Annotations,
		daemonSet.Labels,
		daemonSet.Spec.Template.Annotations,
		daemonSet.Spec.Template.Labels,
		daemonSet.Spec.Template.Spec,
	)
}

func WorkloadFromPod(opts Options, pod *corev1.Pod) graph.WorkloadInput {
	return workloadFromPodSpec(
		opts,
		"v1",
		"Pod",
		pod.Namespace,
		pod.Name,
		string(pod.UID),
		pod.Annotations,
		pod.Labels,
		nil,
		nil,
		pod.Spec,
	)
}

func WorkloadFromJob(opts Options, job *batchv1.Job) graph.WorkloadInput {
	return workloadFromPodSpec(
		opts,
		"batch/v1",
		"Job",
		job.Namespace,
		job.Name,
		string(job.UID),
		job.Annotations,
		job.Labels,
		job.Spec.Template.Annotations,
		job.Spec.Template.Labels,
		job.Spec.Template.Spec,
	)
}

func WorkloadFromCronJob(opts Options, cronJob *batchv1.CronJob) graph.WorkloadInput {
	return workloadFromPodSpec(
		opts,
		"batch/v1",
		"CronJob",
		cronJob.Namespace,
		cronJob.Name,
		string(cronJob.UID),
		cronJob.Annotations,
		cronJob.Labels,
		cronJob.Spec.JobTemplate.Spec.Template.Annotations,
		cronJob.Spec.JobTemplate.Spec.Template.Labels,
		cronJob.Spec.JobTemplate.Spec.Template.Spec,
	)
}

func workloadFromPodSpec(
	opts Options,
	apiVersion string,
	kind string,
	namespace string,
	name string,
	uid string,
	objectAnnotations map[string]string,
	objectLabels map[string]string,
	templateAnnotations map[string]string,
	templateLabels map[string]string,
	podSpec corev1.PodSpec,
) graph.WorkloadInput {
	annotations := effectiveAnnotations(objectAnnotations, templateAnnotations, opts.ReadPodTemplateAnnotations)
	labels := effectiveLabels(objectLabels, templateLabels, opts.ReadPodTemplateAnnotations)
	labelKeys := opts.MetadataLabelKeys.withDefaults()
	mode := discoveryModeFor(annotations, opts.DefaultMode)

	ref := graph.WorkloadRef{
		Cluster:    opts.Cluster,
		Namespace:  namespace,
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		UID:        uid,
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

	ignore := ignoredSecrets(opts.Cluster, namespace, annotations)
	edges := []graph.DependencyEdge{}

	if mode == graph.DiscoveryModeInfer || mode == graph.DiscoveryModeHybrid {
		edges = append(edges, inferPodSpecEdges(opts, ref, podSpec)...)
	}

	if mode == graph.DiscoveryModeExplicit || mode == graph.DiscoveryModeHybrid {
		edges = append(edges, explicitAnnotationEdges(opts.Cluster, ref, annotations)...)
	}

	input.Edges = filterAndMergeEdges(edges, ignore)
	return input
}

func inferPodSpecEdges(opts Options, workload graph.WorkloadRef, podSpec corev1.PodSpec) []graph.DependencyEdge {
	edges := []graph.DependencyEdge{}

	for _, container := range podSpec.Containers {
		edges = append(edges, containerEdges(opts.Cluster, workload, container.Name, "", false, container.Env, container.EnvFrom)...)
	}

	if opts.IncludeInitContainers {
		for _, container := range podSpec.InitContainers {
			edges = append(edges, containerEdges(opts.Cluster, workload, container.Name, "init", false, container.Env, container.EnvFrom)...)
		}
	}

	if opts.IncludeEphemeralContainers {
		for _, container := range podSpec.EphemeralContainers {
			edges = append(edges, containerEdges(opts.Cluster, workload, container.Name, "", true, container.Env, container.EnvFrom)...)
		}
	}

	for _, volume := range podSpec.Volumes {
		if volume.Secret == nil || volume.Secret.SecretName == "" {
			continue
		}

		edges = append(edges, newEdge(
			opts.Cluster,
			workload,
			workload.Namespace,
			volume.Secret.SecretName,
			volume.Secret.Optional != nil && *volume.Secret.Optional,
			graph.DiscoveryModeInfer,
			graph.DependencySource{
				Type:   "volumes.secret",
				Path:   fmt.Sprintf("spec.volumes[%s].secret.secretName", volume.Name),
				Volume: volume.Name,
			},
		))
	}

	if opts.IncludeImagePullSecrets {
		for i, ref := range podSpec.ImagePullSecrets {
			if ref.Name == "" {
				continue
			}

			edges = append(edges, newEdge(
				opts.Cluster,
				workload,
				workload.Namespace,
				ref.Name,
				false,
				graph.DiscoveryModeInfer,
				graph.DependencySource{
					Type: "imagePullSecrets",
					Path: fmt.Sprintf("spec.imagePullSecrets[%d].name", i),
				},
			))
		}
		if len(podSpec.ImagePullSecrets) == 0 {
			serviceAccountName := serviceAccountNameForPodSpec(podSpec.ServiceAccountName)
			for i, secretName := range serviceAccountImagePullSecretNames(opts, workload.Namespace, serviceAccountName) {
				edges = append(edges, newEdge(
					opts.Cluster,
					workload,
					workload.Namespace,
					secretName,
					false,
					graph.DiscoveryModeInfer,
					graph.DependencySource{
						Type:          "serviceAccount.imagePullSecrets",
						Path:          fmt.Sprintf("serviceAccount[%s].imagePullSecrets[%d].name", serviceAccountName, i),
						ResourceField: "imagePullSecrets",
					},
				))
			}
		}

	}

	return edges
}

func ServiceAccountKey(namespace, name string) string {
	return namespace + "/" + serviceAccountNameForPodSpec(name)
}

func serviceAccountNameForPodSpec(name string) string {
	if strings.TrimSpace(name) == "" {
		return "default"
	}
	return name
}

func serviceAccountImagePullSecretNames(opts Options, namespace, serviceAccountName string) []string {
	if len(opts.ServiceAccountImagePullSecrets) == 0 {
		return nil
	}
	return opts.ServiceAccountImagePullSecrets[ServiceAccountKey(namespace, serviceAccountName)]
}

func containerEdges(
	cluster string,
	workload graph.WorkloadRef,
	containerName string,
	containerRole string,
	ephemeral bool,
	env []corev1.EnvVar,
	envFrom []corev1.EnvFromSource,
) []graph.DependencyEdge {
	edges := []graph.DependencyEdge{}

	for _, item := range env {
		if item.ValueFrom == nil || item.ValueFrom.SecretKeyRef == nil || item.ValueFrom.SecretKeyRef.Name == "" {
			continue
		}

		selector := item.ValueFrom.SecretKeyRef
		source := graph.DependencySource{
			Type:      "env.secretKeyRef",
			Path:      fmt.Sprintf("env[%s].valueFrom.secretKeyRef[%s#%s]", item.Name, selector.Name, selector.Key),
			Container: containerName,
			EnvVar:    item.Name,
		}

		if containerRole == "init" {
			source.Container = ""
			source.InitContainer = containerName
		}
		if ephemeral {
			source.Ephemeral = true
		}

		edges = append(edges, newEdge(
			cluster,
			workload,
			workload.Namespace,
			selector.Name,
			selector.Optional != nil && *selector.Optional,
			graph.DiscoveryModeInfer,
			source,
		))
	}

	for _, item := range envFrom {
		if item.SecretRef == nil || item.SecretRef.Name == "" {
			continue
		}

		source := graph.DependencySource{
			Type:      "envFrom.secretRef",
			Path:      fmt.Sprintf("envFrom.secretRef[%s]", item.SecretRef.Name),
			Container: containerName,
		}

		if containerRole == "init" {
			source.Container = ""
			source.InitContainer = containerName
		}
		if ephemeral {
			source.Ephemeral = true
		}

		edges = append(edges, newEdge(
			cluster,
			workload,
			workload.Namespace,
			item.SecretRef.Name,
			item.SecretRef.Optional != nil && *item.SecretRef.Optional,
			graph.DiscoveryModeInfer,
			source,
		))
	}

	return edges
}

func explicitAnnotationEdges(cluster string, workload graph.WorkloadRef, annotations map[string]string) []graph.DependencyEdge {
	edges := []graph.DependencyEdge{}

	if value := strings.TrimSpace(annotations[AnnotationWatchSecrets]); value != "" {
		refs, err := ParseWatchSecrets(cluster, workload.Namespace, value)
		if err == nil {
			for _, ref := range refs {
				edges = append(edges, newEdge(
					cluster,
					workload,
					ref.Namespace,
					ref.Name,
					false,
					graph.DiscoveryModeExplicit,
					graph.DependencySource{
						Type:       "annotation",
						Path:       fmt.Sprintf("metadata.annotations[%s]", AnnotationWatchSecrets),
						Annotation: AnnotationWatchSecrets,
					},
				))
			}
		}
	}

	if value := strings.TrimSpace(annotations[AnnotationWatchSecretsJSON]); value != "" {
		var tokens []string
		if err := json.Unmarshal([]byte(value), &tokens); err == nil {
			for _, token := range tokens {
				ref, err := parseSecretRef(cluster, workload.Namespace, token)
				if err != nil {
					continue
				}
				edges = append(edges, newEdge(
					cluster,
					workload,
					ref.Namespace,
					ref.Name,
					false,
					graph.DiscoveryModeExplicit,
					graph.DependencySource{
						Type:       "annotation",
						Path:       fmt.Sprintf("metadata.annotations[%s]", AnnotationWatchSecretsJSON),
						Annotation: AnnotationWatchSecretsJSON,
					},
				))
			}
		}
	}

	return edges
}

func newEdge(
	cluster string,
	workload graph.WorkloadRef,
	secretNamespace string,
	secretName string,
	optional bool,
	mode graph.DiscoveryMode,
	source graph.DependencySource,
) graph.DependencyEdge {
	secret := graph.SecretRef{
		Cluster:   cluster,
		Namespace: secretNamespace,
		Name:      secretName,
	}

	edge := graph.DependencyEdge{
		Cluster:       cluster,
		Workload:      workload,
		Secret:        secret,
		DiscoveryMode: mode,
		Sources:       []graph.DependencySource{source},
		Optional:      optional,
		OwnerTeam:     "",
	}
	edge.ID = graph.EdgeID(workload, secret)
	return edge
}

func filterAndMergeEdges(edges []graph.DependencyEdge, ignore map[string]struct{}) []graph.DependencyEdge {
	merged := map[string]graph.DependencyEdge{}

	for _, edge := range edges {
		if _, ignored := ignore[graph.SecretID(edge.Secret)]; ignored {
			continue
		}

		if existing, ok := merged[edge.ID]; ok {
			existing.Sources = append(existing.Sources, edge.Sources...)
			existing.Optional = existing.Optional && edge.Optional
			if existing.DiscoveryMode != edge.DiscoveryMode {
				existing.DiscoveryMode = graph.DiscoveryModeHybrid
			}
			merged[edge.ID] = existing
			continue
		}

		merged[edge.ID] = edge
	}

	out := make([]graph.DependencyEdge, 0, len(merged))
	for _, edge := range merged {
		out = append(out, edge)
	}

	return out
}

func ignoredSecrets(cluster, namespace string, annotations map[string]string) map[string]struct{} {
	result := map[string]struct{}{}
	value := strings.TrimSpace(annotations[AnnotationIgnoreSecrets])
	if value == "" {
		return result
	}

	refs, err := ParseWatchSecrets(cluster, namespace, value)
	if err != nil {
		return result
	}

	for _, ref := range refs {
		result[graph.SecretID(ref)] = struct{}{}
	}

	return result
}

func effectiveAnnotations(objectAnnotations, templateAnnotations map[string]string, includeTemplate bool) map[string]string {
	out := map[string]string{}

	if includeTemplate {
		for k, v := range templateAnnotations {
			out[k] = v
		}
	}

	for k, v := range objectAnnotations {
		out[k] = v
	}

	return out
}

func effectiveLabels(objectLabels, templateLabels map[string]string, includeTemplate bool) map[string]string {
	out := map[string]string{}

	if includeTemplate {
		for k, v := range templateLabels {
			out[k] = v
		}
	}

	for k, v := range objectLabels {
		out[k] = v
	}

	return out
}

func metadataValue(annotationValue string, labels map[string]string, labelKeys []string, labelsEnabled bool) string {
	if value := strings.TrimSpace(annotationValue); value != "" {
		return value
	}

	if !labelsEnabled {
		return ""
	}

	return firstLabelValue(labels, labelKeys)
}

func firstLabelValue(labels map[string]string, keys []string) string {
	for _, key := range keys {
		value := strings.TrimSpace(labels[key])
		if value != "" {
			return value
		}
	}

	return ""
}

func (c MetadataLabelKeyConfig) withDefaults() MetadataLabelKeyConfig {
	defaults := DefaultMetadataLabelKeyConfig()

	if len(c.OwnerTeam) == 0 {
		c.OwnerTeam = defaults.OwnerTeam
	}
	if len(c.Service) == 0 {
		c.Service = defaults.Service
	}
	if len(c.Environment) == 0 {
		c.Environment = defaults.Environment
	}
	if len(c.Criticality) == 0 {
		c.Criticality = defaults.Criticality
	}

	return c
}

func discoveryModeFor(annotations map[string]string, fallback graph.DiscoveryMode) graph.DiscoveryMode {
	if fallback == "" {
		fallback = graph.DiscoveryModeHybrid
	}

	if value, ok := annotations[AnnotationEnabled]; ok && isFalse(value) {
		return graph.DiscoveryModeDisabled
	}

	switch strings.ToLower(strings.TrimSpace(annotations[AnnotationDiscoveryMode])) {
	case "infer":
		return graph.DiscoveryModeInfer
	case "explicit":
		return graph.DiscoveryModeExplicit
	case "hybrid":
		return graph.DiscoveryModeHybrid
	case "disabled":
		return graph.DiscoveryModeDisabled
	default:
		return fallback
	}
}

func isFalse(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "no", "off", "disabled":
		return true
	default:
		return false
	}
}

func HasControllerOwner(refs []metav1.OwnerReference) bool {
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller {
			return true
		}
	}
	return false
}

func HasControllerOwnerKind(refs []metav1.OwnerReference, kind string) bool {
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller && ref.Kind == kind {
			return true
		}
	}
	return false
}
