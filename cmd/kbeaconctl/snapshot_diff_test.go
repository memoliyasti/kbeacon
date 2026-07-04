package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSnapshotDiffText(t *testing.T) {
	oldPath, newPath := writeSnapshotDiffFixtures(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runSnapshotDiff([]string{oldPath, newPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}

	output := stdout.String()

	for _, expected := range []string{
		"KBeacon Snapshot Diff",
		"Secrets: +0 -0 ~1",
		"Workloads: +1 -0 ~0",
		"Edges: +0 -0 ~1",
		"payments/db",
		"payments/deployment/worker",
		"payments/deployment/api->payments/db",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected text output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunSnapshotDiffJSON(t *testing.T) {
	oldPath, newPath := writeSnapshotDiffFixtures(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runSnapshotDiff([]string{"--format", "json", "--include", "secrets,workloads", oldPath, newPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}

	var decoded snapshotDiffDocument
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json diff: %v\n%s", err, stdout.String())
	}

	if decoded.Kind != "KBeaconSnapshotDiff" {
		t.Fatalf("expected KBeaconSnapshotDiff, got %q", decoded.Kind)
	}

	if !decoded.Summary.HasChanges {
		t.Fatal("expected diff to report changes")
	}

	if len(decoded.Resources) != 2 {
		t.Fatalf("expected two resources, got %d", len(decoded.Resources))
	}
}

func TestRunSnapshotDiffMarkdown(t *testing.T) {
	oldPath, newPath := writeSnapshotDiffFixtures(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runSnapshotDiff([]string{"--format", "markdown", oldPath, newPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}

	output := stdout.String()

	for _, expected := range []string{
		"## KBeacon Snapshot Diff",
		"| Resource | Added | Removed | Changed |",
		"| Secrets | 0 | 0 | 1 |",
		"| Workloads | 1 | 0 | 0 |",
		"| Edges | 0 | 0 | 1 |",
		"<details>",
		"`payments/deployment/worker`",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected markdown output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunSnapshotDiffFailOnChange(t *testing.T) {
	oldPath, newPath := writeSnapshotDiffFixtures(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runSnapshotDiff([]string{"--fail-on-change", oldPath, newPath}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 for changed snapshots, got %d", code)
	}
}

func TestRunSnapshotDiffFailOnChangeNoChange(t *testing.T) {
	oldPath, _ := writeSnapshotDiffFixtures(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runSnapshotDiff([]string{"--fail-on-change", oldPath, oldPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 for identical snapshots, got %d stderr=%s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "No snapshot changes detected.") {
		t.Fatalf("expected no-change text output, got:\n%s", stdout.String())
	}
}

func writeSnapshotDiffFixtures(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()

	oldSnapshot := map[string]any{
		"apiVersion": "kbeacon.io/v1",
		"kind":       "KBeaconSnapshot",
		"resources": map[string]any{
			"secrets": map[string]any{
				"data": []any{
					map[string]any{
						"ref": map[string]any{
							"namespace": "payments",
							"name":      "db",
						},
						"impactScore": 10,
					},
				},
			},
			"workloads": map[string]any{
				"data": []any{
					map[string]any{
						"ref": map[string]any{
							"namespace": "payments",
							"kind":      "Deployment",
							"name":      "api",
						},
						"dependencyCount": 1,
					},
				},
			},
			"dependency-map": map[string]any{
				"data": map[string]any{
					"edges": []any{
						map[string]any{
							"workload": map[string]any{
								"namespace": "payments",
								"kind":      "Deployment",
								"name":      "api",
							},
							"secret": map[string]any{
								"namespace": "payments",
								"name":      "db",
							},
							"resolved": true,
						},
					},
				},
			},
		},
	}

	newSnapshot := map[string]any{
		"apiVersion": "kbeacon.io/v1",
		"kind":       "KBeaconSnapshot",
		"resources": map[string]any{
			"secrets": map[string]any{
				"data": []any{
					map[string]any{
						"ref": map[string]any{
							"namespace": "payments",
							"name":      "db",
						},
						"impactScore": 42,
					},
				},
			},
			"workloads": map[string]any{
				"data": []any{
					map[string]any{
						"ref": map[string]any{
							"namespace": "payments",
							"kind":      "Deployment",
							"name":      "api",
						},
						"dependencyCount": 1,
					},
					map[string]any{
						"ref": map[string]any{
							"namespace": "payments",
							"kind":      "Deployment",
							"name":      "worker",
						},
						"dependencyCount": 0,
					},
				},
			},
			"dependency-map": map[string]any{
				"data": map[string]any{
					"edges": []any{
						map[string]any{
							"workload": map[string]any{
								"namespace": "payments",
								"kind":      "Deployment",
								"name":      "api",
							},
							"secret": map[string]any{
								"namespace": "payments",
								"name":      "db",
							},
							"resolved": false,
						},
					},
				},
			},
		},
	}

	oldPath := filepath.Join(dir, "old-snapshot.json")
	newPath := filepath.Join(dir, "new-snapshot.json")

	writeSnapshotDiffFixture(t, oldPath, oldSnapshot)
	writeSnapshotDiffFixture(t, newPath, newSnapshot)

	return oldPath, newPath
}

func writeSnapshotDiffFixture(t *testing.T, path string, value map[string]any) {
	t.Helper()

	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal snapshot fixture: %v", err)
	}

	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write snapshot fixture: %v", err)
	}
}
