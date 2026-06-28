package discovery

import (
	"testing"

	"github.com/memoliyasti/kbeacon/internal/graph"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
