package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/memoliyasti/kbeacon/internal/graph"
)

func TestLoadConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	err := os.WriteFile(path, []byte(`
apiVersion: kbeacon.io/v1alpha1
kind: AgentConfig
cluster:
  name: yaml-cluster
log:
  level: debug
agent:
  http:
    port: 9099
  shutdownGracePeriod: 20s
discovery:
  defaultMode: infer
  includeImagePullSecrets: false
  includeInitContainers: true
  includeEphemeralContainers: false
  readPodTemplateAnnotations: true
  metadataLabels:
    enabled: true
    ownerTeam:
      - team
    service:
      - service
    environment:
      - env
    criticality:
      - priority
  namespaces:
    include:
      - app-a
      - app-b
    exclude: []
  resyncInterval: 1h
  reconcile:
    debounce: 500ms
metrics:
  edge:
    enabled: true
  runtime:
    enabled: true
resourcesToWatch:
  externalSecrets:
    externalSecrets: true
privacy:
  redaction:
    secretKeys: true
`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Cluster.Name != "yaml-cluster" {
		t.Fatalf("expected cluster yaml-cluster, got %q", cfg.Cluster.Name)
	}

	if cfg.Log.Level != "debug" {
		t.Fatalf("expected log level debug, got %q", cfg.Log.Level)
	}

	if cfg.HTTPBindAddress() != ":9099" {
		t.Fatalf("expected bind address :9099, got %q", cfg.HTTPBindAddress())
	}

	if cfg.DiscoveryMode() != graph.DiscoveryModeInfer {
		t.Fatalf("expected discovery mode infer, got %q", cfg.DiscoveryMode())
	}

	if len(cfg.Discovery.Namespaces.Include) != 2 {
		t.Fatalf("expected 2 included namespaces, got %#v", cfg.Discovery.Namespaces.Include)
	}

	if !cfg.Discovery.MetadataLabels.Enabled {
		t.Fatal("expected metadata label fallback to be enabled")
	}

	if got := cfg.Discovery.MetadataLabels.OwnerTeam[0]; got != "team" {
		t.Fatalf("expected custom owner team label key team, got %q", got)
	}

	resync, err := cfg.ResyncInterval()
	if err != nil {
		t.Fatalf("ResyncInterval returned error: %v", err)
	}
	if resync.String() != "1h0m0s" {
		t.Fatalf("expected resync 1h, got %s", resync)
	}

	debounce, err := cfg.RebuildDebounce()
	if err != nil {
		t.Fatalf("RebuildDebounce returned error: %v", err)
	}
	if debounce.String() != "500ms" {
		t.Fatalf("expected debounce 500ms, got %s", debounce)
	}

	if !cfg.Privacy.Redaction.SecretKeys {
		t.Fatalf("expected Secret key redaction from YAML")
	}

	if !cfg.ResourcesToWatch.ExternalSecrets.ExternalSecrets {
		t.Fatalf("expected ExternalSecret watcher from YAML")
	}
}

func TestLoadConfigEnvOverrides(t *testing.T) {
	t.Setenv("KBEACON_CLUSTER_NAME", "env-cluster")
	t.Setenv("KBEACON_INCLUDE_NAMESPACES", "team-a,team-b")
	t.Setenv("KBEACON_EXCLUDE_NAMESPACES", "kube-system,kube-public")
	t.Setenv("KBEACON_REDACT_SECRET_KEYS", "true")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Cluster.Name != "env-cluster" {
		t.Fatalf("expected env cluster override, got %q", cfg.Cluster.Name)
	}

	if len(cfg.Discovery.Namespaces.Include) != 2 {
		t.Fatalf("expected include namespaces from env, got %#v", cfg.Discovery.Namespaces.Include)
	}

	if len(cfg.Discovery.Namespaces.Exclude) != 2 {
		t.Fatalf("expected exclude namespaces from env, got %#v", cfg.Discovery.Namespaces.Exclude)
	}

	if !cfg.Privacy.Redaction.SecretKeys {
		t.Fatalf("expected Secret key redaction from env")
	}
}
