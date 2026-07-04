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
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}

	output := stdout.String()
	expected := []string{
		"KBeacon Snapshot Diff",
		"Secrets: +0 -0 ~1",
		"Workloads: +1 -0 ~0",
		"Edges: +0 -0 ~1",
		"~ payments/db",
		"+ payments/deployment/worker",
	}

	for _, item := range expected {
		if !strings.Contains(output, item) {
			t.Fatalf("expected output to contain %q, got:\n%s", item, output)
		}
	}
}

func TestRunSnapshotDiffJSON(t *testing.T) {
	oldPath, newPath := writeSnapshotDiffFixtures(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runSnapshotDiff([]string{"--format", "json", "--include", "secrets,workloads", oldPath, newPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", code, stderr.String())
	}

	var decoded snapshotDiffDocument
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json diff: %v\n%s", err, stdout.String())
	}

	if decoded.Kind != "KBeaconSnapshotDiff" {
		t.Fatalf("expected KBeaconSnapshotDiff, got %q", decoded.Kind)
	}

	if decoded.Summary["secrets"].Changed != 1 {
		t.Fatalf("expected one changed Secret, got %#v", decoded.Summary["secrets"])
	}

	if decoded.Summary["workloads"].Added != 1 {
		t.Fatalf("expected one added workload, got %#v", decoded.Summary["workloads"])
	}

	if _, ok := decoded.Summary["edges"]; ok {
		t.Fatalf("did not expect edge summary when edges are not included: %#v", decoded.Summary)
	}
}

func TestRunSnapshotDiffFailOnChange(t *testing.T) {
	oldPath, newPath := writeSnapshotDiffFixtures(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runSnapshotDiff([]string{"--fail-on-change", oldPath, newPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 when changes are present, got %d", code)
	}
}

func writeSnapshotDiffFixtures(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.json")
	newPath := filepath.Join(dir, "new.json")

	oldSnapshot := `{
  "apiVersion": "kbeacon.io/v1",
  "kind": "KBeaconSnapshot",
  "cluster": "ci",
  "resources": {
    "secrets": {
      "data": [
        {
          "ref": {
            "namespace": "payments",
            "name": "db"
          },
          "exists": true,
          "impactScore": 10
        }
      ]
    },
    "workloads": {
      "data": [
        {
          "ref": {
            "namespace": "payments",
            "kind": "Deployment",
            "name": "api"
          },
          "dependencyCount": 1
        }
      ]
    },
    "dependency-map": {
      "data": {
        "edges": [
          {
            "id": "payments/deployment/api->payments/db",
            "resolved": true
          }
        ]
      }
    }
  }
}`

	newSnapshot := `{
  "apiVersion": "kbeacon.io/v1",
  "kind": "KBeaconSnapshot",
  "cluster": "ci",
  "resources": {
    "secrets": {
      "data": [
        {
          "ref": {
            "namespace": "payments",
            "name": "db"
          },
          "exists": true,
          "impactScore": 20
        }
      ]
    },
    "workloads": {
      "data": [
        {
          "ref": {
            "namespace": "payments",
            "kind": "Deployment",
            "name": "api"
          },
          "dependencyCount": 1
        },
        {
          "ref": {
            "namespace": "payments",
            "kind": "Deployment",
            "name": "worker"
          },
          "dependencyCount": 0
        }
      ]
    },
    "dependency-map": {
      "data": {
        "edges": [
          {
            "id": "payments/deployment/api->payments/db",
            "resolved": false
          }
        ]
      }
    }
  }
}`

	writeFileForSnapshotDiffTest(t, oldPath, oldSnapshot)
	writeFileForSnapshotDiffTest(t, newPath, newSnapshot)

	return oldPath, newPath
}

func writeFileForSnapshotDiffTest(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
