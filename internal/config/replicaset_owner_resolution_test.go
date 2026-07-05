package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultEnablesReplicaSetOwnerResolution(t *testing.T) {
	cfg := Default()
	if !cfg.ResourcesToWatch.Apps.ReplicaSets {
		t.Fatal("expected ReplicaSet owner-resolution cache to be enabled by default")
	}
}

func TestLoadConfigParsesReplicaSetWatcher(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	err := os.WriteFile(path, []byte(`
cluster:
  name: yaml-cluster
resourcesToWatch:
  apps:
    replicaSets: false
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.ResourcesToWatch.Apps.ReplicaSets {
		t.Fatal("expected ReplicaSet watcher to be disabled from YAML")
	}
}
