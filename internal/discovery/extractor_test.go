package discovery

import (
	"testing"

	"github.com/memoliyasti/kbeacon/internal/graph"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWorkloadFromDeploymentExtractsSecretDependencies(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "kbeacon-demo",
			UID:       "deployment-uid",
			Annotations: map[string]string{
				AnnotationEnabled:       "true",
				AnnotationDiscoveryMode: "hybrid",
				AnnotationOwnerTeam:     "platform",
				AnnotationCriticality:   "high",
				AnnotationWatchSecrets:  "shared/api-token",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "pull-secret"},
					},
					Containers: []corev1.Container{
						{
							Name: "api",
							Env: []corev1.EnvVar{
								{
									Name: "DB_USERNAME",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "db-secret",
											},
											Key: "username",
										},
									},
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "envfrom-secret",
										},
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "volume-secret",
								},
							},
						},
					},
				},
			},
		},
	}

	opts := DefaultOptions("minikube")
	input := WorkloadFromDeployment(opts, deployment)

	if input.Ref.Cluster != "minikube" ||
		input.Ref.Namespace != "kbeacon-demo" ||
		input.Ref.Kind != "Deployment" ||
		input.Ref.Name != "api" {
		t.Fatalf("unexpected workload ref: %#v", input.Ref)
	}

	if input.OwnerTeam != "platform" {
		t.Fatalf("expected owner team platform, got %q", input.OwnerTeam)
	}

	if input.Criticality != "high" {
		t.Fatalf("expected criticality high, got %q", input.Criticality)
	}

	edgesBySecret := map[string]graph.DependencyEdge{}
	for _, edge := range input.Edges {
		edgesBySecret[graph.SecretID(edge.Secret)] = edge
	}

	expectedSecrets := []string{
		"kbeacon-demo/db-secret",
		"kbeacon-demo/envfrom-secret",
		"kbeacon-demo/volume-secret",
		"kbeacon-demo/pull-secret",
		"shared/api-token",
	}

	for _, key := range expectedSecrets {
		if _, ok := edgesBySecret[key]; !ok {
			t.Fatalf("missing dependency edge for secret %s; edges=%#v", key, input.Edges)
		}
	}

	if edgesBySecret["kbeacon-demo/db-secret"].DiscoveryMode != graph.DiscoveryModeInfer {
		t.Fatalf("expected db-secret to be inferred, got %s", edgesBySecret["kbeacon-demo/db-secret"].DiscoveryMode)
	}

	if edgesBySecret["shared/api-token"].DiscoveryMode != graph.DiscoveryModeExplicit {
		t.Fatalf("expected shared/api-token to be explicit, got %s", edgesBySecret["shared/api-token"].DiscoveryMode)
	}
}

func TestWorkloadFromDeploymentDisabled(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "kbeacon-demo",
			Annotations: map[string]string{
				AnnotationEnabled: "false",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "api",
							Env: []corev1.EnvVar{
								{
									Name: "DB_USERNAME",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "db-secret",
											},
											Key: "username",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	input := WorkloadFromDeployment(DefaultOptions("minikube"), deployment)

	if input.DiscoveryMode != graph.DiscoveryModeDisabled {
		t.Fatalf("expected disabled discovery mode, got %s", input.DiscoveryMode)
	}

	if len(input.Edges) != 0 {
		t.Fatalf("expected no edges for disabled workload, got %#v", input.Edges)
	}
}

func TestWorkloadFromDeploymentUsesMetadataLabelFallback(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "payments",
			UID:       "deployment-uid",
			Labels: map[string]string{
				"app.kubernetes.io/team":        "payments-platform",
				"app.kubernetes.io/name":        "payments",
				"app.kubernetes.io/environment": "prod",
				"priority":                      "critical",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "api",
							Env: []corev1.EnvVar{
								{
									Name: "DB_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "payments-db",
											},
											Key: "password",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	input := WorkloadFromDeployment(DefaultOptions("minikube"), deployment)

	if input.OwnerTeam != "payments-platform" {
		t.Fatalf("expected owner team from labels, got %q", input.OwnerTeam)
	}
	if input.Service != "payments" {
		t.Fatalf("expected service from labels, got %q", input.Service)
	}
	if input.Environment != "prod" {
		t.Fatalf("expected environment from labels, got %q", input.Environment)
	}
	if input.Criticality != "critical" {
		t.Fatalf("expected criticality from labels, got %q", input.Criticality)
	}
}

func TestWorkloadMetadataAnnotationsOverrideLabels(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "payments",
			UID:       "deployment-uid",
			Labels: map[string]string{
				"app.kubernetes.io/team":        "label-team",
				"app.kubernetes.io/name":        "label-service",
				"app.kubernetes.io/environment": "label-env",
				"priority":                      "low",
			},
			Annotations: map[string]string{
				AnnotationOwnerTeam:   "annotation-team",
				AnnotationService:     "annotation-service",
				AnnotationEnvironment: "annotation-env",
				AnnotationCriticality: "high",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "api",
							Command: []string{"sh", "-c", "sleep 3600"},
						},
					},
				},
			},
		},
	}

	input := WorkloadFromDeployment(DefaultOptions("minikube"), deployment)

	if input.OwnerTeam != "annotation-team" {
		t.Fatalf("expected annotation owner team to win, got %q", input.OwnerTeam)
	}
	if input.Service != "annotation-service" {
		t.Fatalf("expected annotation service to win, got %q", input.Service)
	}
	if input.Environment != "annotation-env" {
		t.Fatalf("expected annotation environment to win, got %q", input.Environment)
	}
	if input.Criticality != "high" {
		t.Fatalf("expected annotation criticality to win, got %q", input.Criticality)
	}
}

func TestWorkloadFromDeploymentUsesServiceAccountImagePullSecretsFallback(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "kbeacon-demo",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "builder",
					Containers: []corev1.Container{
						{
							Name:    "api",
							Command: []string{"sh", "-c", "sleep 3600"},
						},
					},
				},
			},
		},
	}

	opts := DefaultOptions("minikube")
	opts.ServiceAccountImagePullSecrets = map[string][]string{
		ServiceAccountKey("kbeacon-demo", "builder"): []string{"builder-pull-secret"},
	}

	input := WorkloadFromDeployment(opts, deployment)

	edgesBySecret := map[string]graph.DependencyEdge{}
	for _, edge := range input.Edges {
		edgesBySecret[graph.SecretID(edge.Secret)] = edge
	}

	edge, ok := edgesBySecret["kbeacon-demo/builder-pull-secret"]
	if !ok {
		t.Fatalf("missing ServiceAccount imagePullSecrets fallback edge; edges=%#v", input.Edges)
	}

	if edge.DiscoveryMode != graph.DiscoveryModeInfer {
		t.Fatalf("expected fallback edge to be inferred, got %s", edge.DiscoveryMode)
	}

	if len(edge.Sources) != 1 || edge.Sources[0].Type != "serviceAccount.imagePullSecrets" {
		t.Fatalf("unexpected fallback edge sources: %#v", edge.Sources)
	}
}

func TestWorkloadFromDeploymentPrefersExplicitPodImagePullSecretsOverServiceAccountFallback(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "kbeacon-demo",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "builder",
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "explicit-pull-secret"},
					},
					Containers: []corev1.Container{
						{
							Name:    "api",
							Command: []string{"sh", "-c", "sleep 3600"},
						},
					},
				},
			},
		},
	}

	opts := DefaultOptions("minikube")
	opts.ServiceAccountImagePullSecrets = map[string][]string{
		ServiceAccountKey("kbeacon-demo", "builder"): []string{"builder-pull-secret"},
	}

	input := WorkloadFromDeployment(opts, deployment)

	edgesBySecret := map[string]graph.DependencyEdge{}
	for _, edge := range input.Edges {
		edgesBySecret[graph.SecretID(edge.Secret)] = edge
	}

	if _, ok := edgesBySecret["kbeacon-demo/explicit-pull-secret"]; !ok {
		t.Fatalf("expected explicit Pod imagePullSecrets edge, got %#v", input.Edges)
	}

	if _, ok := edgesBySecret["kbeacon-demo/builder-pull-secret"]; ok {
		t.Fatalf("did not expect ServiceAccount fallback when Pod spec imagePullSecrets is set; edges=%#v", input.Edges)
	}
}

func TestWorkloadFromIngressExtractsTLSEdges(t *testing.T) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "payments-web",
			Namespace: "payments",
			UID:       "ingress-uid",
			Annotations: map[string]string{
				AnnotationOwnerTeam:   "payments-platform",
				AnnotationCriticality: "high",
			},
			Labels: map[string]string{
				"app.kubernetes.io/name": "payments-web",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{
				{Hosts: []string{"payments.example.com"}, SecretName: "payments-web-tls"},
				{Hosts: []string{"api.payments.example.com"}, SecretName: "payments-api-tls"},
			},
		},
	}

	input := WorkloadFromIngress(DefaultOptions("minikube"), ingress)

	if input.Ref.Cluster != "minikube" || input.Ref.Namespace != "payments" || input.Ref.APIVersion != "networking.k8s.io/v1" || input.Ref.Kind != "Ingress" || input.Ref.Name != "payments-web" {
		t.Fatalf("unexpected ingress workload ref: %#v", input.Ref)
	}

	edgesBySecret := map[string]graph.DependencyEdge{}
	for _, edge := range input.Edges {
		edgesBySecret[graph.SecretID(edge.Secret)] = edge
	}

	for _, key := range []string{"payments/payments-web-tls", "payments/payments-api-tls"} {
		edge, ok := edgesBySecret[key]
		if !ok {
			t.Fatalf("missing Ingress TLS edge for %s; edges=%#v", key, input.Edges)
		}
		if edge.DiscoveryMode != graph.DiscoveryModeInfer {
			t.Fatalf("expected Ingress TLS edge to be inferred, got %s", edge.DiscoveryMode)
		}
		if len(edge.Sources) != 1 || edge.Sources[0].Type != "ingress.tls" {
			t.Fatalf("unexpected Ingress TLS edge sources: %#v", edge.Sources)
		}
	}
}

func TestWorkloadFromDeploymentExtractsProjectedSecretVolumeDependencies(t *testing.T) {
	optional := true

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "kbeacon-demo",
			UID:       "deployment-uid",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "api",
							Image: "busybox:1.36",
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "projected-config",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									Sources: []corev1.VolumeProjection{
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "projected-secret",
												},
												Optional: &optional,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	input := WorkloadFromDeployment(DefaultOptions("minikube"), deployment)

	for _, edge := range input.Edges {
		if edge.Secret.Namespace != "kbeacon-demo" || edge.Secret.Name != "projected-secret" {
			continue
		}

		if !edge.Optional {
			t.Fatalf("expected projected Secret edge to preserve optional=true: %#v", edge)
		}

		if len(edge.Sources) != 1 {
			t.Fatalf("expected one source for projected Secret edge, got %#v", edge.Sources)
		}

		source := edge.Sources[0]
		if source.Type != "volumes.projected.sources.secret" {
			t.Fatalf("expected projected Secret source type, got %q", source.Type)
		}

		if source.Volume != "projected-config" {
			t.Fatalf("expected source volume projected-config, got %q", source.Volume)
		}

		if source.Path != "spec.volumes[projected-config].projected.sources[0].secret.name" {
			t.Fatalf("unexpected source path: %q", source.Path)
		}

		return
	}

	t.Fatalf("expected projected Secret dependency edge, got %#v", input.Edges)
}
