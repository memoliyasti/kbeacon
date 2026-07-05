package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/memoliyasti/kbeacon/internal/graph"
)

func TestCompatibilityAliasesMatchV1Endpoints(t *testing.T) {
	handler := New(Options{
		Cluster: "compat-cluster",
		Graph:   compatibilityAliasGraph(t),
		Now: func() time.Time {
			return time.Date(2026, 7, 5, 12, 30, 0, 0, time.UTC)
		},
	})

	for _, tc := range []struct {
		name  string
		v1    string
		alias string
	}{
		{
			name:  "secrets list",
			v1:    "/api/v1/secrets",
			alias: "/api/secrets",
		},
		{
			name:  "workloads list",
			v1:    "/api/v1/workloads",
			alias: "/api/workloads",
		},
		{
			name:  "dependency map",
			v1:    "/api/v1/dependency-map",
			alias: "/api/dependency-map",
		},
		{
			name:  "secret impact",
			v1:    "/api/v1/secrets/payments/db/impact",
			alias: "/api/secrets/payments/db/impact",
		},
		{
			name:  "workload dependencies",
			v1:    "/api/v1/workloads/payments/Deployment/api/dependencies",
			alias: "/api/workloads/payments/Deployment/api/dependencies",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v1 := compatibilityResponseBody(t, handler, tc.v1)
			alias := compatibilityResponseBody(t, handler, tc.alias)

			if !bytes.Equal(v1, alias) {
				t.Fatalf("compatibility alias response differs from canonical v1 endpoint\nv1=%s\nalias=%s", string(v1), string(alias))
			}
		})
	}
}

func compatibilityAliasGraph(t *testing.T) *graph.Cache {
	t.Helper()

	cache := graph.NewCache("compat-cluster")
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)

	workload := graph.WorkloadRef{
		Cluster:    "compat-cluster",
		Namespace:  "payments",
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "api",
		UID:        "deployment-api-uid",
	}
	secret := graph.SecretRef{
		Cluster:   "compat-cluster",
		Namespace: "payments",
		Name:      "db",
	}

	cache.ApplySnapshot(
		[]graph.SecretInput{
			{
				Ref:               secret,
				Type:              "Opaque",
				OwnerTeam:         "platform",
				Criticality:       "high",
				ResourceVersion:   "1",
				CreationTimestamp: now,
			},
		},
		[]graph.WorkloadInput{
			{
				Ref:           workload,
				OwnerTeam:     "platform",
				Service:       "payments-api",
				Environment:   "prod",
				Criticality:   "high",
				DiscoveryMode: graph.DiscoveryModeHybrid,
				Edges: []graph.DependencyEdge{
					{
						Workload:      workload,
						Secret:        secret,
						DiscoveryMode: graph.DiscoveryModeInfer,
						Optional:      false,
						Sources: []graph.DependencySource{
							{
								Type:      "env.secretKeyRef",
								Path:      "spec.template.spec.containers[api].env[DB_PASSWORD].valueFrom.secretKeyRef",
								Container: "api",
								EnvVar:    "DB_PASSWORD",
							},
						},
					},
				},
			},
		},
		now,
	)

	return cache
}

func compatibilityResponseBody(t *testing.T, handler http.Handler, path string) []byte {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s returned status %d body=%s", path, recorder.Code, recorder.Body.String())
	}

	return bytes.TrimSpace(recorder.Body.Bytes())
}
