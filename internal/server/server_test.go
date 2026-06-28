package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/memoliyasti/kbeacon/internal/graph"
)

func newTestServer() http.Handler {
	cache := graph.NewCache("test-cluster")
	now := time.Unix(1000, 0).UTC()

	workload := graph.WorkloadRef{
		Cluster:    "test-cluster",
		Namespace:  "payments",
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "payments-api",
	}

	secret := graph.SecretRef{
		Cluster:   "test-cluster",
		Namespace: "payments",
		Name:      "payments-db",
	}

	cache.ApplySnapshot(
		[]graph.SecretInput{
			{
				Ref:               secret,
				Type:              "Opaque",
				OwnerTeam:         "payments-platform",
				Criticality:       "critical",
				ResourceVersion:   "1",
				CreationTimestamp: now,
			},
		},
		[]graph.WorkloadInput{
			{
				Ref:           workload,
				OwnerTeam:     "payments-platform",
				Service:       "payments-api",
				Environment:   "prod",
				Criticality:   "critical",
				DiscoveryMode: graph.DiscoveryModeHybrid,
				Edges: []graph.DependencyEdge{
					{
						Workload:      workload,
						Secret:        secret,
						DiscoveryMode: graph.DiscoveryModeInfer,
						Sources: []graph.DependencySource{
							{
								Type:      "env.secretKeyRef",
								Path:      "env[DB_PASSWORD].valueFrom.secretKeyRef[payments-db#password]",
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

	return New(Options{
		Cluster: "test-cluster",
		Version: "test",
		Commit:  "test",
		Now: func() time.Time {
			return now
		},
		Graph: cache,
	})
}

func getJSON(t *testing.T, h http.Handler, path string) map[string]any {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s returned status %d body=%s", path, rec.Code, rec.Body.String())
	}

	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response for %s: %v body=%s", path, err, rec.Body.String())
	}

	return out
}

func TestDependencyMapResponseShape(t *testing.T) {
	h := newTestServer()
	out := getJSON(t, h, "/api/v1/dependency-map")

	data := out["data"].(map[string]any)

	if _, ok := data["nodes"].([]any); !ok {
		t.Fatalf("expected dependency map data.nodes array, got %#v", data)
	}

	edges, ok := data["edges"].([]any)
	if !ok || len(edges) != 1 {
		t.Fatalf("expected one dependency map edge, got %#v", data["edges"])
	}

	edge := edges[0].(map[string]any)
	if _, ok := edge["workload"].(map[string]any); !ok {
		t.Fatalf("expected edge.workload object, got %#v", edge)
	}
	if _, ok := edge["secret"].(map[string]any); !ok {
		t.Fatalf("expected edge.secret object, got %#v", edge)
	}

	if _, ok := edge["from"]; ok {
		t.Fatalf("dependency edges must not expose deprecated from field: %#v", edge)
	}
	if _, ok := edge["to"]; ok {
		t.Fatalf("dependency edges must not expose deprecated to field: %#v", edge)
	}
}

func TestSecretImpactResponseShape(t *testing.T) {
	h := newTestServer()
	out := getJSON(t, h, "/api/v1/secrets/payments/payments-db/impact")

	data := out["data"].(map[string]any)

	if _, ok := data["secret"].(map[string]any); !ok {
		t.Fatalf("expected secret impact data.secret object, got %#v", data)
	}
	if _, ok := data["summary"].(map[string]any); !ok {
		t.Fatalf("expected secret impact data.summary object, got %#v", data)
	}
	if _, ok := data["affectedTeams"].([]any); !ok {
		t.Fatalf("expected secret impact data.affectedTeams array, got %#v", data)
	}
	if _, ok := data["affectedWorkloads"].([]any); !ok {
		t.Fatalf("expected secret impact data.affectedWorkloads array, got %#v", data)
	}
	if _, ok := data["edges"].([]any); !ok {
		t.Fatalf("expected secret impact data.edges array, got %#v", data)
	}
	if _, ok := data["workloads"]; ok {
		t.Fatalf("secret impact must not expose deprecated workloads field: %#v", data)
	}
}

func TestWorkloadDependenciesResponseShape(t *testing.T) {
	h := newTestServer()
	out := getJSON(t, h, "/api/v1/workloads/payments/Deployment/payments-api/dependencies")

	data := out["data"].(map[string]any)

	if _, ok := data["workload"].(map[string]any); !ok {
		t.Fatalf("expected workload dependencies data.workload object, got %#v", data)
	}

	deps, ok := data["dependencies"].([]any)
	if !ok || len(deps) != 1 {
		t.Fatalf("expected workload dependencies data.dependencies array, got %#v", data)
	}

	dep := deps[0].(map[string]any)
	if _, ok := dep["secret"].(map[string]any); !ok {
		t.Fatalf("expected dependency.secret object, got %#v", dep)
	}
	if dep["discoveryMode"] == "" {
		t.Fatalf("expected dependency.discoveryMode, got %#v", dep)
	}

	if _, ok := data["secrets"]; ok {
		t.Fatalf("workload dependencies must not expose deprecated secrets field: %#v", data)
	}
	if _, ok := data["edges"]; ok {
		t.Fatalf("workload dependencies must not expose deprecated edges field: %#v", data)
	}
}

func TestListFilters(t *testing.T) {
	h := newTestServer()

	secrets := getJSON(t, h, "/api/v1/secrets?namespace=payments&ownerTeam=payments-platform&criticality=critical")
	if got := len(secrets["data"].([]any)); got != 1 {
		t.Fatalf("expected one filtered secret, got %d", got)
	}

	emptySecrets := getJSON(t, h, "/api/v1/secrets?namespace=missing")
	if got := len(emptySecrets["data"].([]any)); got != 0 {
		t.Fatalf("expected zero filtered secrets, got %d", got)
	}

	workloads := getJSON(t, h, "/api/v1/workloads?workloadKind=deployment")
	if got := len(workloads["data"].([]any)); got != 1 {
		t.Fatalf("expected case-insensitive workloadKind filter to match, got %d", got)
	}
}

func TestUnknownSubresourcesReturn404(t *testing.T) {
	h := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/payments/payments-db/unknown", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "unknown Secret API path") {
		t.Fatalf("expected unknown Secret API path error, got %s", rec.Body.String())
	}
}
