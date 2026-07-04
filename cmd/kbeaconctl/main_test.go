package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "kbeaconctl version=") {
		t.Fatalf("unexpected version output: %q", stdout.String())
	}
}

func TestRunHealthUsesConfiguredServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("expected /healthz, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"--server", server.URL, "health"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), `"status":"ok"`) {
		t.Fatalf("unexpected health output: %q", stdout.String())
	}
}

func TestRunGetSecretsWithFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/secrets" {
			t.Fatalf("expected /api/v1/secrets, got %s", r.URL.Path)
		}

		q := r.URL.Query()
		if q.Get("namespace") != "payments" {
			t.Fatalf("expected namespace filter, got query %s", r.URL.RawQuery)
		}
		if q.Get("exists") != "true" {
			t.Fatalf("expected exists filter, got query %s", r.URL.RawQuery)
		}
		if q.Get("limit") != "5" {
			t.Fatalf("expected limit filter, got query %s", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"--server", server.URL,
		"get", "secrets",
		"--namespace", "payments",
		"--exists", "true",
		"--limit", "5",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), `"data":[]`) {
		t.Fatalf("unexpected get output: %q", stdout.String())
	}
}

func TestRunImpactJSON(t *testing.T) {
	server := impactTestServer(t)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"--server", server.URL, "impact", "payments", "payments-db"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}

	normalized := strings.NewReplacer(" ", "", "\n", "", "\t", "").Replace(stdout.String())
	if !strings.Contains(normalized, `"affectedWorkloadCount":1`) {
		t.Fatalf("unexpected impact output: %q", stdout.String())
	}
}

func TestRunImpactReportSubcommand(t *testing.T) {
	server := impactTestServer(t)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"--server", server.URL, "impact", "report", "payments", "payments-db"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}

	report := stdout.String()
	for _, expected := range []string{
		"KBeacon Secret Impact Report",
		"Secret: payments/payments-db",
		"Impact score: 42.50",
		"Affected workloads: 1",
		"Discovery modes: hybrid=1",
		"payments-platform: 1 workload(s)",
		"Deployment payments/payments-api",
		"Deployment payments/payments-api -> Secret payments/payments-db mode=hybrid resolved=yes optional=no",
		"sources: env.secretKeyRef",
	} {
		if !strings.Contains(report, expected) {
			t.Fatalf("expected report to contain %q, got:\n%s", expected, report)
		}
	}
}

func TestRunImpactReportFormatFlag(t *testing.T) {
	server := impactTestServer(t)
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"--server", server.URL, "impact", "--format", "report", "payments", "payments-db"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "KBeacon Secret Impact Report") {
		t.Fatalf("expected human-readable report, got %q", stdout.String())
	}
}

func TestRunUnknownGetResource(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"get", "widgets"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown resource")
	}

	if !strings.Contains(stderr.String(), "unknown get resource") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRunSnapshotExport(t *testing.T) {
	requested := map[string]bool{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.URL.Path] = true
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/config":
			_, _ = w.Write([]byte(`{"apiVersion":"kbeacon.io/v1","data":{"cluster":{"name":"ci"}}}`))
		case "/api/v1/secrets":
			_, _ = w.Write([]byte(`{"apiVersion":"kbeacon.io/v1","data":[{"ref":{"namespace":"payments","name":"payments-db"}}]}`))
		case "/api/v1/workloads":
			_, _ = w.Write([]byte(`{"apiVersion":"kbeacon.io/v1","data":[{"ref":{"namespace":"payments","kind":"Deployment","name":"payments-api"}}]}`))
		case "/api/v1/dependency-map":
			_, _ = w.Write([]byte(`{"apiVersion":"kbeacon.io/v1","data":{"nodes":[],"edges":[]}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"--server", server.URL, "snapshot", "export"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}

	for _, path := range []string{
		"/api/v1/config",
		"/api/v1/secrets",
		"/api/v1/workloads",
		"/api/v1/dependency-map",
	} {
		if !requested[path] {
			t.Fatalf("expected snapshot export to request %s; requested=%v", path, requested)
		}
	}

	output := stdout.String()
	for _, expected := range []string{
		`"kind": "KBeaconSnapshot"`,
		`"cluster": "ci"`,
		`"config"`,
		`"secrets"`,
		`"workloads"`,
		`"dependencyMap"`,
		`"payments-db"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected snapshot output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunSnapshotExportIncludeSubset(t *testing.T) {
	requested := map[string]bool{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested[r.URL.Path] = true
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/secrets":
			_, _ = w.Write([]byte(`{"apiVersion":"kbeacon.io/v1","data":[]}`))
		case "/api/v1/dependency-map":
			_, _ = w.Write([]byte(`{"apiVersion":"kbeacon.io/v1","data":{"nodes":[],"edges":[]}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"--server", server.URL,
		"snapshot", "export",
		"--include", "secrets,dependency-map",
		"--pretty=false",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}

	if requested["/api/v1/config"] || requested["/api/v1/workloads"] {
		t.Fatalf("unexpected extra snapshot requests: %v", requested)
	}

	output := stdout.String()
	if !strings.Contains(output, `"kind":"KBeaconSnapshot"`) {
		t.Fatalf("expected compact snapshot JSON, got %q", output)
	}
	if !strings.Contains(output, `"dependencyMap"`) {
		t.Fatalf("expected dependencyMap resource, got %q", output)
	}
}

func impactTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/secrets/payments/payments-db/impact" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"apiVersion": "kbeacon.io/v1",
			"cluster": "ci",
			"generatedAt": "2026-07-03T17:00:00Z",
			"data": {
				"secret": {
					"ref": {
						"cluster": "ci",
						"namespace": "payments",
						"name": "payments-db"
					},
					"exists": true,
					"type": "Opaque",
					"ownerTeam": "payments-platform",
					"criticality": "critical",
					"affectedWorkloadCount": 1,
					"affectedTeamCount": 1,
					"affectedNamespaceCount": 1,
					"unresolvedReferenceCount": 0,
					"impactScore": 42.5
				},
				"summary": {
					"affectedWorkloadCount": 1,
					"affectedTeamCount": 1,
					"affectedNamespaceCount": 1,
					"unresolvedReferenceCount": 0,
					"discoveryModes": {
						"hybrid": 1
					}
				},
				"affectedTeams": [
					{
						"ownerTeam": "payments-platform",
						"workloadCount": 1
					}
				],
				"affectedWorkloads": [
					{
						"ref": {
							"cluster": "ci",
							"namespace": "payments",
							"apiVersion": "apps/v1",
							"kind": "Deployment",
							"name": "payments-api"
						},
						"ownerTeam": "payments-platform",
						"criticality": "critical",
						"discoveryMode": "hybrid",
						"dependencyCount": 1,
						"unresolvedCount": 0
					}
				],
				"edges": [
					{
						"workload": {
							"cluster": "ci",
							"namespace": "payments",
							"apiVersion": "apps/v1",
							"kind": "Deployment",
							"name": "payments-api"
						},
						"secret": {
							"cluster": "ci",
							"namespace": "payments",
							"name": "payments-db"
						},
						"discoveryMode": "hybrid",
						"sources": [
							{
								"type": "env.secretKeyRef",
								"path": "env[DB_PASSWORD].valueFrom.secretKeyRef[payments-db#password]"
							}
						],
						"optional": false,
						"resolved": true
					}
				]
			}
		}`))
	}))
}
