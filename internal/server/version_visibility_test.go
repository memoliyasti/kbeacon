package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/memoliyasti/kbeacon/internal/graph"
)

func TestAgentVersionVisibility(t *testing.T) {
	handler := New(Options{
		Cluster: "test-cluster",
		Version: "test-version",
		Commit:  "test-commit",
		Now: func() time.Time {
			return time.Unix(1700000000, 0).UTC()
		},
		Graph: graph.NewCache("test-cluster"),
		Readiness: func() ([]map[string]any, bool) {
			return []map[string]any{
				{"resource": "Secret", "synced": true},
			}, true
		},
	})

	tests := []struct {
		name      string
		endpoint  string
		assertion func(t *testing.T, body map[string]any)
	}{
		{
			name:     "healthz",
			endpoint: "/healthz",
			assertion: func(t *testing.T, body map[string]any) {
				assertTopLevelBuildInfo(t, "/healthz", body)
			},
		},
		{
			name:     "readyz",
			endpoint: "/readyz",
			assertion: func(t *testing.T, body map[string]any) {
				assertTopLevelBuildInfo(t, "/readyz", body)
			},
		},
		{
			name:     "api discovery",
			endpoint: "/api/v1",
			assertion: func(t *testing.T, body map[string]any) {
				assertTopLevelBuildInfo(t, "/api/v1", body)
			},
		},
		{
			name:     "config",
			endpoint: "/api/v1/config",
			assertion: func(t *testing.T, body map[string]any) {
				data, ok := body["data"].(map[string]any)
				if !ok {
					t.Fatalf("/api/v1/config data is not an object: %#v", body["data"])
				}

				agent, ok := data["agent"].(map[string]any)
				if !ok {
					t.Fatalf("/api/v1/config data.agent is not an object: %#v", data["agent"])
				}

				assertBuildValue(t, "/api/v1/config data.agent.version", agent["version"], "test-version")
				assertBuildValue(t, "/api/v1/config data.agent.commit", agent["commit"], "test-commit")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.endpoint, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code < 200 || rec.Code >= 300 {
				t.Fatalf("%s returned status %d: %s", tt.endpoint, rec.Code, rec.Body.String())
			}

			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode %s response: %v body=%s", tt.endpoint, err, rec.Body.String())
			}

			tt.assertion(t, body)
		})
	}
}

func assertTopLevelBuildInfo(t *testing.T, endpoint string, body map[string]any) {
	t.Helper()

	assertBuildValue(t, endpoint+" version", body["version"], "test-version")
	assertBuildValue(t, endpoint+" commit", body["commit"], "test-commit")
}

func assertBuildValue(t *testing.T, field string, got any, want string) {
	t.Helper()

	gotString, ok := got.(string)
	if !ok {
		t.Fatalf("%s is not a string: %#v", field, got)
	}
	if gotString != want {
		t.Fatalf("%s = %q, want %q", field, gotString, want)
	}
}
