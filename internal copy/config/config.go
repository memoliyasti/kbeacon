package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kbeacon/kbeacon/internal/graph"
	"sigs.k8s.io/yaml"
)

type Config struct {
	APIVersion       string                 `yaml:"apiVersion" json:"apiVersion,omitempty"`
	Kind             string                 `yaml:"kind" json:"kind,omitempty"`
	Cluster          ClusterConfig          `yaml:"cluster" json:"cluster"`
	Log              LogConfig              `yaml:"log" json:"log"`
	Agent            AgentConfig            `yaml:"agent" json:"agent"`
	Discovery        DiscoveryConfig        `yaml:"discovery" json:"discovery"`
	Metrics          MetricsConfig          `yaml:"metrics" json:"metrics"`
	ResourcesToWatch ResourcesToWatchConfig `yaml:"resourcesToWatch" json:"resourcesToWatch"`
}

type ClusterConfig struct {
	Name        string `yaml:"name" json:"name"`
	Environment string `yaml:"environment" json:"environment,omitempty"`
	Region      string `yaml:"region" json:"region,omitempty"`
}

type LogConfig struct {
	Level              string `yaml:"level" json:"level"`
	Format             string `yaml:"format" json:"format"`
	IncludeSecretNames bool   `yaml:"includeSecretNames" json:"includeSecretNames"`
}

type AgentConfig struct {
	HTTP                HTTPConfig `yaml:"http" json:"http"`
	ShutdownGracePeriod string     `yaml:"shutdownGracePeriod" json:"shutdownGracePeriod"`
}

type HTTPConfig struct {
	Port        int    `yaml:"port" json:"port"`
	MetricsPath string `yaml:"metricsPath" json:"metricsPath"`
}

type DiscoveryConfig struct {
	DefaultMode                string                  `yaml:"defaultMode" json:"defaultMode"`
	IncludeImagePullSecrets    bool                    `yaml:"includeImagePullSecrets" json:"includeImagePullSecrets"`
	IncludeInitContainers      bool                    `yaml:"includeInitContainers" json:"includeInitContainers"`
	IncludeEphemeralContainers bool                    `yaml:"includeEphemeralContainers" json:"includeEphemeralContainers"`
	ReadPodTemplateAnnotations bool                    `yaml:"readPodTemplateAnnotations" json:"readPodTemplateAnnotations"`
	Namespaces                 NamespaceSelectorConfig `yaml:"namespaces" json:"namespaces"`
	ResyncInterval             string                  `yaml:"resyncInterval" json:"resyncInterval"`
	Reconcile                  ReconcileConfig         `yaml:"reconcile" json:"reconcile"`
}

type NamespaceSelectorConfig struct {
	Include []string `yaml:"include" json:"include"`
	Exclude []string `yaml:"exclude" json:"exclude"`
}

type ReconcileConfig struct {
	Debounce string `yaml:"debounce" json:"debounce"`
	Workers  int    `yaml:"workers" json:"workers"`
}

type MetricsConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	Edge    struct {
		Enabled bool `yaml:"enabled" json:"enabled"`
	} `yaml:"edge" json:"edge"`
	Runtime struct {
		Enabled bool `yaml:"enabled" json:"enabled"`
	} `yaml:"runtime" json:"runtime"`
}

type ResourcesToWatchConfig struct {
	Core struct {
		Secrets bool `yaml:"secrets" json:"secrets"`
		Pods    bool `yaml:"pods" json:"pods"`
	} `yaml:"core" json:"core"`
	Apps struct {
		Deployments  bool `yaml:"deployments" json:"deployments"`
		StatefulSets bool `yaml:"statefulSets" json:"statefulSets"`
		DaemonSets   bool `yaml:"daemonSets" json:"daemonSets"`
		ReplicaSets  bool `yaml:"replicaSets" json:"replicaSets"`
	} `yaml:"apps" json:"apps"`
	Batch struct {
		Jobs     bool `yaml:"jobs" json:"jobs"`
		CronJobs bool `yaml:"cronJobs" json:"cronJobs"`
	} `yaml:"batch" json:"batch"`
}

func Default() Config {
	cfg := Config{
		APIVersion: "kbeacon.io/v1alpha1",
		Kind:       "AgentConfig",
	}

	cfg.Cluster.Name = "local-dev"

	cfg.Log.Level = "info"
	cfg.Log.Format = "json"

	cfg.Agent.HTTP.Port = 8080
	cfg.Agent.HTTP.MetricsPath = "/metrics"
	cfg.Agent.ShutdownGracePeriod = "15s"

	cfg.Discovery.DefaultMode = string(graph.DiscoveryModeHybrid)
	cfg.Discovery.IncludeImagePullSecrets = true
	cfg.Discovery.IncludeInitContainers = true
	cfg.Discovery.IncludeEphemeralContainers = true
	cfg.Discovery.ReadPodTemplateAnnotations = true
	cfg.Discovery.Namespaces.Exclude = []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
	}
	cfg.Discovery.ResyncInterval = "10h"
	cfg.Discovery.Reconcile.Debounce = "250ms"
	cfg.Discovery.Reconcile.Workers = 1

	cfg.Metrics.Enabled = true
	cfg.Metrics.Edge.Enabled = true
	cfg.Metrics.Runtime.Enabled = true

	cfg.ResourcesToWatch.Core.Secrets = true
	cfg.ResourcesToWatch.Core.Pods = true
	cfg.ResourcesToWatch.Apps.Deployments = true
	cfg.ResourcesToWatch.Apps.StatefulSets = true
	cfg.ResourcesToWatch.Apps.DaemonSets = true
	cfg.ResourcesToWatch.Apps.ReplicaSets = true
	cfg.ResourcesToWatch.Batch.Jobs = true
	cfg.ResourcesToWatch.Batch.CronJobs = true

	return cfg
}

func Load(path string) (Config, error) {
	cfg := Default()

	if strings.TrimSpace(path) == "" {
		cfg.ApplyEnv()
		return cfg, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file %q: %w", path, err)
	}

	cfg.Normalize()
	cfg.ApplyEnv()

	return cfg, nil
}

func (c *Config) ApplyEnv() {
	if value := os.Getenv("KBEACON_CLUSTER_NAME"); value != "" {
		c.Cluster.Name = value
	}
	if value := os.Getenv("KBEACON_LOG_LEVEL"); value != "" {
		c.Log.Level = value
	}
	if value := os.Getenv("KBEACON_RESYNC_INTERVAL"); value != "" {
		c.Discovery.ResyncInterval = value
	}
	if value := os.Getenv("KBEACON_REBUILD_DEBOUNCE"); value != "" {
		c.Discovery.Reconcile.Debounce = value
	}
	if value := os.Getenv("KBEACON_INCLUDE_NAMESPACES"); value != "" {
		c.Discovery.Namespaces.Include = SplitCSV(value)
	}
	if value := os.Getenv("KBEACON_EXCLUDE_NAMESPACES"); value != "" {
		c.Discovery.Namespaces.Exclude = SplitCSV(value)
	}
	if value := os.Getenv("KBEACON_INCLUDE_IMAGE_PULL_SECRETS"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			c.Discovery.IncludeImagePullSecrets = parsed
		}
	}
}

func (c *Config) Normalize() {
	if c.Cluster.Name == "" {
		c.Cluster.Name = "local-dev"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Agent.HTTP.Port == 0 {
		c.Agent.HTTP.Port = 8080
	}
	if c.Agent.HTTP.MetricsPath == "" {
		c.Agent.HTTP.MetricsPath = "/metrics"
	}
	if c.Agent.ShutdownGracePeriod == "" {
		c.Agent.ShutdownGracePeriod = "15s"
	}
	if c.Discovery.DefaultMode == "" {
		c.Discovery.DefaultMode = string(graph.DiscoveryModeHybrid)
	}
	if c.Discovery.ResyncInterval == "" {
		c.Discovery.ResyncInterval = "10h"
	}
	if c.Discovery.Reconcile.Debounce == "" {
		c.Discovery.Reconcile.Debounce = "250ms"
	}
	if len(c.Discovery.Namespaces.Exclude) == 0 {
		c.Discovery.Namespaces.Exclude = []string{
			"kube-system",
			"kube-public",
			"kube-node-lease",
		}
	}
}

func (c Config) HTTPBindAddress() string {
	return fmt.Sprintf(":%d", c.Agent.HTTP.Port)
}

func (c Config) ResyncInterval() (time.Duration, error) {
	return time.ParseDuration(c.Discovery.ResyncInterval)
}

func (c Config) RebuildDebounce() (time.Duration, error) {
	return time.ParseDuration(c.Discovery.Reconcile.Debounce)
}

func (c Config) ShutdownGracePeriod() (time.Duration, error) {
	return time.ParseDuration(c.Agent.ShutdownGracePeriod)
}

func (c Config) DiscoveryMode() graph.DiscoveryMode {
	switch strings.ToLower(strings.TrimSpace(c.Discovery.DefaultMode)) {
	case "infer":
		return graph.DiscoveryModeInfer
	case "explicit":
		return graph.DiscoveryModeExplicit
	case "disabled":
		return graph.DiscoveryModeDisabled
	default:
		return graph.DiscoveryModeHybrid
	}
}

func SplitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}

	return out
}
