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

func newRichTestServer() http.Handler {
	cache := graph.NewCache("test-cluster")
	now := time.Unix(1000, 0).UTC()

	paymentsAPI := graph.WorkloadRef{Cluster: "test-cluster", Namespace: "payments", APIVersion: "apps/v1", Kind: "Deployment", Name: "payments-api"}
	paymentJob := graph.WorkloadRef{Cluster: "test-cluster", Namespace: "payments", APIVersion: "batch/v1", Kind: "Job", Name: "payment-reconciler"}
	reportsAPI := graph.WorkloadRef{Cluster: "test-cluster", Namespace: "reports", APIVersion: "apps/v1", Kind: "Deployment", Name: "reports-api"}

	paymentsDB := graph.SecretRef{Cluster: "test-cluster", Namespace: "payments", Name: "payments-db"}
	legacyToken := graph.SecretRef{Cluster: "test-cluster", Namespace: "payments", Name: "legacy-payment-token"}
	platformCA := graph.SecretRef{Cluster: "test-cluster", Namespace: "shared", Name: "platform-ca"}

	cache.ApplySnapshot(
		[]graph.SecretInput{
			{Ref: paymentsDB, Type: "Opaque", OwnerTeam: "payments-platform", Criticality: "critical", ResourceVersion: "1", CreationTimestamp: now},
			{Ref: platformCA, Type: "Opaque", OwnerTeam: "platform", Criticality: "high", ResourceVersion: "1", CreationTimestamp: now},
		},
		[]graph.WorkloadInput{
			{
				Ref:           paymentsAPI,
				OwnerTeam:     "payments-platform",
				Service:       "payments",
				Environment:   "prod",
				Criticality:   "critical",
				DiscoveryMode: graph.DiscoveryModeHybrid,
				Edges: []graph.DependencyEdge{
					{Workload: paymentsAPI, Secret: paymentsDB, DiscoveryMode: graph.DiscoveryModeInfer, Sources: []graph.DependencySource{{Type: "env.secretKeyRef", Path: "env[DB_PASSWORD].valueFrom.secretKeyRef[payments-db#password]", Container: "api", EnvVar: "DB_PASSWORD"}}},
					{Workload: paymentsAPI, Secret: legacyToken, DiscoveryMode: graph.DiscoveryModeExplicit, Sources: []graph.DependencySource{{Type: "annotation", Path: "metadata.annotations[kbeacon.io/watch-secrets]", Annotation: "kbeacon.io/watch-secrets"}}},
				},
			},
			{
				Ref:           paymentJob,
				OwnerTeam:     "payments-platform",
				Service:       "payments",
				Environment:   "prod",
				Criticality:   "high",
				DiscoveryMode: graph.DiscoveryModeInfer,
				Edges: []graph.DependencyEdge{
					{Workload: paymentJob, Secret: platformCA, DiscoveryMode: graph.DiscoveryModeInfer, Sources: []graph.DependencySource{{Type: "volumes.secret", Path: "spec.volumes[ca].secret.secretName"}}},
				},
			},
			{
				Ref:           reportsAPI,
				OwnerTeam:     "data-platform",
				Service:       "reports",
				Environment:   "prod",
				Criticality:   "medium",
				DiscoveryMode: graph.DiscoveryModeExplicit,
				Edges: []graph.DependencyEdge{
					{Workload: reportsAPI, Secret: paymentsDB, DiscoveryMode: graph.DiscoveryModeExplicit, Sources: []graph.DependencySource{{Type: "annotation", Path: "metadata.annotations[kbeacon.io/watch-secrets]", Annotation: "kbeacon.io/watch-secrets"}}},
				},
			},
		},
		now,
	)

	return New(Options{Cluster: "test-cluster", Version: "test", Commit: "test", Now: func() time.Time { return now }, Graph: cache})
}

func getJSON(t *testing.T, h http.Handler, path string) map[string]any {
	t.Helper()
	return getJSONStatus(t, h, path, http.StatusOK)
}

func getJSONStatus(t *testing.T, h http.Handler, path string, status int) map[string]any {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != status {
		t.Fatalf("GET %s returned status %d, expected %d, body=%s", path, rec.Code, status, rec.Body.String())
	}

	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response for %s: %v body=%s", path, err, rec.Body.String())
	}

	return out
}

func pagination(t *testing.T, out map[string]any) map[string]any {
	t.Helper()

	p, ok := out["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("expected pagination object, got %#v", out["pagination"])
	}
	return p
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

func TestSecretsPaginationAndFilters(t *testing.T) {
	h := newRichTestServer()

	out := getJSON(t, h, "/api/v1/secrets?limit=1&offset=1")
	data := out["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("expected one paged secret, got %d", len(data))
	}

	page := pagination(t, out)
	if page["limit"].(float64) != 1 || page["offset"].(float64) != 1 || page["total"].(float64) != 3 || page["returned"].(float64) != 1 {
		t.Fatalf("unexpected pagination: %#v", page)
	}
	if page["nextOffset"].(float64) != 2 {
		t.Fatalf("expected nextOffset 2, got %#v", page)
	}

	unresolved := getJSON(t, h, "/api/v1/secrets?exists=false")
	unresolvedItems := unresolved["data"].([]any)
	if len(unresolvedItems) != 1 {
		t.Fatalf("expected one unresolved secret, got %d", len(unresolvedItems))
	}

	ref := unresolvedItems[0].(map[string]any)["ref"].(map[string]any)
	if ref["name"] != "legacy-payment-token" {
		t.Fatalf("expected legacy-payment-token, got %#v", ref)
	}

	paymentsDB := getJSON(t, h, "/api/v1/secrets?secretName=payments-db&ownerTeam=payments-platform")
	if got := len(paymentsDB["data"].([]any)); got != 1 {
		t.Fatalf("expected one payments-db secret, got %d", got)
	}
}

func TestWorkloadsPaginationAndFilters(t *testing.T) {
	h := newRichTestServer()

	out := getJSON(t, h, "/api/v1/workloads?namespace=payments&workloadKind=deployment&workloadName=payments-api&discoveryMode=hybrid")
	items := out["data"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected one filtered workload, got %d", len(items))
	}

	item := items[0].(map[string]any)
	if item["ownerTeam"] != "payments-platform" {
		t.Fatalf("expected payments-platform owner team, got %#v", item)
	}

	capped := getJSON(t, h, "/api/v1/workloads?limit=5000")
	page := pagination(t, capped)
	if page["limit"].(float64) != maxLimit {
		t.Fatalf("expected capped limit %d, got %#v", maxLimit, page)
	}
}

func TestDependencyMapPaginationAndFilters(t *testing.T) {
	h := newRichTestServer()

	out := getJSON(t, h, "/api/v1/dependency-map?secretName=payments-db&resolved=true&limit=10")
	data := out["data"].(map[string]any)

	edges := data["edges"].([]any)
	if len(edges) != 2 {
		t.Fatalf("expected two resolved payments-db edges, got %d: %#v", len(edges), edges)
	}

	nodes := data["nodes"].([]any)
	if len(nodes) != 3 {
		t.Fatalf("expected three nodes for two edges sharing one secret, got %d: %#v", len(nodes), nodes)
	}

	page := pagination(t, out)
	if page["total"].(float64) != 2 || page["returned"].(float64) != 2 {
		t.Fatalf("unexpected dependency-map pagination: %#v", page)
	}

	unresolved := getJSON(t, h, "/api/v1/dependency-map?resolved=false")
	unresolvedData := unresolved["data"].(map[string]any)
	unresolvedEdges := unresolvedData["edges"].([]any)
	if len(unresolvedEdges) != 1 {
		t.Fatalf("expected one unresolved edge, got %d: %#v", len(unresolvedEdges), unresolvedEdges)
	}

	edge := unresolvedEdges[0].(map[string]any)
	secret := edge["secret"].(map[string]any)
	if secret["name"] != "legacy-payment-token" {
		t.Fatalf("expected legacy-payment-token unresolved edge, got %#v", edge)
	}
}

func TestInvalidPaginationAndBooleanFiltersReturn400(t *testing.T) {
	h := newRichTestServer()

	out := getJSONStatus(t, h, "/api/v1/secrets?limit=abc", http.StatusBadRequest)
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "invalid_query" {
		t.Fatalf("expected invalid_query, got %#v", errObj)
	}

	out = getJSONStatus(t, h, "/api/v1/dependency-map?resolved=maybe", http.StatusBadRequest)
	errObj = out["error"].(map[string]any)
	if errObj["code"] != "invalid_query" {
		t.Fatalf("expected invalid_query, got %#v", errObj)
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
