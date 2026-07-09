package server

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/memoliyasti/kbeacon/internal/discovery"
	"github.com/memoliyasti/kbeacon/internal/graph"
	kmetrics "github.com/memoliyasti/kbeacon/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestAgentOutputsDoNotExposeSecretValues(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	cluster := "test-cluster"

	secretValues := []string{
		"sentinel-alpha-should-not-leak",
		"sentinel-bravo-should-not-leak",
		"sentinel-charlie-should-not-leak",
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "payments-db",
			Namespace:         "payments",
			ResourceVersion:   "rv-1",
			CreationTimestamp: metav1.NewTime(now),
			Annotations: map[string]string{
				discovery.AnnotationOwnerTeam:   "payments-platform",
				discovery.AnnotationCriticality: "critical",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"password": []byte(secretValues[0]),
			"api-key":  []byte(secretValues[1]),
		},
		StringData: map[string]string{
			"connection": secretValues[2],
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "payments-api",
			Namespace: "payments",
			UID:       types.UID("payments-api-uid"),
			Annotations: map[string]string{
				discovery.AnnotationOwnerTeam:     "payments-platform",
				discovery.AnnotationCriticality:   "high",
				discovery.AnnotationDiscoveryMode: "hybrid",
				discovery.AnnotationWatchSecrets:  "shared/platform-ca",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "api",
							Image: "busybox:1.36",
							Env: []corev1.EnvVar{
								{
									Name: "DATABASE_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: "payments-db"},
											Key:                  "password",
										},
									},
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: "payments-db"},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "db-secret",
									MountPath: "/var/run/secrets/db",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "db-secret",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "payments-db",
								},
							},
						},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "registry-pull-secret"},
					},
				},
			},
		},
	}

	cache := graph.NewCache(cluster)
	cache.ApplySnapshot(
		[]graph.SecretInput{
			discovery.SecretInputFromSecret(cluster, secret),
		},
		[]graph.WorkloadInput{
			discovery.WorkloadFromDeployment(discovery.DefaultOptions(cluster), deployment),
		},
		now,
	)

	registry := prometheus.NewRegistry()
	if err := registry.Register(kmetrics.NewGraphCollector(cache, cluster, "test-version", "test-commit")); err != nil {
		t.Fatalf("register graph collector: %v", err)
	}

	handler := New(Options{
		Cluster:        cluster,
		Graph:          cache,
		Now:            func() time.Time { return now },
		MetricsHandler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	})

	endpoints := []string{
		"/api/v1",
		"/api/v1/config",
		"/api/v1/secrets",
		"/api/v1/workloads",
		"/api/v1/dependency-map",
		"/api/v1/secrets/payments/payments-db/impact",
		"/api/v1/workloads/payments/Deployment/payments-api/dependencies",
		"/metrics",
	}

	for _, endpoint := range endpoints {
		body := responseBodyForNoSecretLeakTest(t, handler, endpoint)

		expectedMetadata := map[string]string{
			"/api/v1/secrets":                             "payments-db",
			"/api/v1/workloads":                           "payments-api",
			"/api/v1/dependency-map":                      "payments-db",
			"/api/v1/secrets/payments/payments-db/impact": "payments-db",
			"/api/v1/workloads/payments/Deployment/payments-api/dependencies": "payments-db",
			"/metrics": "payments-db",
		}

		if want := expectedMetadata[endpoint]; want != "" && !bytes.Contains(body, []byte(want)) {
			t.Fatalf("endpoint %s did not include expected metadata %q; response=%s", endpoint, want, string(body))
		}

		for _, secretValue := range secretValues {
			forbidden := []string{
				secretValue,
				base64.StdEncoding.EncodeToString([]byte(secretValue)),
			}

			for _, item := range forbidden {
				if bytes.Contains(body, []byte(item)) {
					t.Fatalf("endpoint %s exposed forbidden Secret value %q in response:\n%s", endpoint, item, string(body))
				}
			}
		}
	}
}

func responseBodyForNoSecretLeakTest(t *testing.T, handler http.Handler, endpoint string) []byte {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, endpoint, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("endpoint %s returned status %d: %s", endpoint, rec.Code, rec.Body.String())
	}

	return rec.Body.Bytes()
}
