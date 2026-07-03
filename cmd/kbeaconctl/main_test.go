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

func TestRunImpact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/secrets/payments/payments-db/impact" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"summary":{"affectedWorkloadCount":2}}}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"--server", server.URL, "impact", "payments", "payments-db"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), `"affectedWorkloadCount":2`) {
		t.Fatalf("unexpected impact output: %q", stdout.String())
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
