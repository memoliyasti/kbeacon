package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type cliConfig struct {
	Server      string `json:"server,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Service     string `json:"service,omitempty"`
	ServicePort string `json:"servicePort,omitempty"`
	Kubeconfig  string `json:"kubeconfig,omitempty"`
	Context     string `json:"context,omitempty"`
}

var cliConfigKeys = []string{
	"context",
	"kubeconfig",
	"namespace",
	"server",
	"service",
	"service-port",
}

func runConfig(configPath string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printConfigUsage(stderr)
		return 2
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "path":
		if len(args) != 1 {
			fmt.Fprintln(stderr, usage("config path"))
			return 2
		}

		path, err := cliConfigPath(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "resolve config path: %v\n", err)
			return 1
		}

		fmt.Fprintln(stdout, path)
		return 0

	case "view", "current":
		if len(args) != 1 {
			fmt.Fprintln(stderr, usage("config view"))
			return 2
		}

		cfg, path, err := loadCLIConfig(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load CLI config: %v\n", err)
			return 1
		}

		out := struct {
			Path   string    `json:"path"`
			Config cliConfig `json:"config"`
		}{
			Path:   path,
			Config: cfg,
		}

		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(stderr, "encode CLI config: %v\n", err)
			return 1
		}

		return 0

	case "get":
		if len(args) != 2 {
			fmt.Fprintln(stderr, usage("config get <key>"))
			return 2
		}

		key, err := normalizeCLIConfigKey(args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		cfg, _, err := loadCLIConfig(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load CLI config: %v\n", err)
			return 1
		}

		fmt.Fprintln(stdout, cliConfigValue(cfg, key))
		return 0

	case "set":
		if len(args) != 3 {
			fmt.Fprintln(stderr, usage("config set <key> <value>"))
			fmt.Fprintf(stderr, "supported keys: %s\n", supportedCLIConfigKeys())
			return 2
		}

		key, err := normalizeCLIConfigKey(args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		value := strings.TrimSpace(args[2])
		if value == "" {
			fmt.Fprintf(stderr, "config value for %s cannot be empty\n", key)
			return 2
		}

		cfg, _, err := loadCLIConfig(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load CLI config: %v\n", err)
			return 1
		}

		setCLIConfigValue(&cfg, key, value)

		path, err := saveCLIConfig(configPath, cfg)
		if err != nil {
			fmt.Fprintf(stderr, "save CLI config: %v\n", err)
			return 1
		}

		fmt.Fprintf(stdout, "set %s=%s in %s\n", key, value, path)
		return 0

	case "unset":
		if len(args) != 2 {
			fmt.Fprintln(stderr, usage("config unset <key>"))
			fmt.Fprintf(stderr, "supported keys: %s\n", supportedCLIConfigKeys())
			return 2
		}

		key, err := normalizeCLIConfigKey(args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}

		cfg, _, err := loadCLIConfig(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load CLI config: %v\n", err)
			return 1
		}

		unsetCLIConfigValue(&cfg, key)

		path, err := saveCLIConfig(configPath, cfg)
		if err != nil {
			fmt.Fprintf(stderr, "save CLI config: %v\n", err)
			return 1
		}

		fmt.Fprintf(stdout, "unset %s in %s\n", key, path)
		return 0

	case "reset":
		if len(args) != 1 {
			fmt.Fprintln(stderr, usage("config reset"))
			return 2
		}

		path, err := cliConfigPath(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "resolve config path: %v\n", err)
			return 1
		}

		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "remove CLI config %s: %v\n", path, err)
			return 1
		}

		fmt.Fprintf(stdout, "reset %s\n", path)
		return 0

	default:
		printConfigUsage(stderr)
		return 2
	}
}

func printConfigUsage(w io.Writer) {
	cmd := commandUsageName()

	fmt.Fprintf(w, "usage: %s config <path|view|get|set|unset|reset> [args]\n", cmd)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintf(w, "  %s config set namespace kbeacon-system\n", cmd)
	fmt.Fprintf(w, "  %s config get namespace\n", cmd)
	fmt.Fprintf(w, "  %s config view\n", cmd)
	fmt.Fprintf(w, "  %s config unset namespace\n", cmd)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Supported keys: %s\n", supportedCLIConfigKeys())
}

func loadCLIConfig(configPath string) (cliConfig, string, error) {
	path, err := cliConfigPath(configPath)
	if err != nil {
		return cliConfig{}, "", err
	}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cliConfig{}, path, nil
	}
	if err != nil {
		return cliConfig{}, path, err
	}

	if strings.TrimSpace(string(raw)) == "" {
		return cliConfig{}, path, nil
	}

	var cfg cliConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cliConfig{}, path, fmt.Errorf("parse %s: %w", path, err)
	}

	return cfg, path, nil
}

func saveCLIConfig(configPath string, cfg cliConfig) (string, error) {
	path, err := cliConfigPath(configPath)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}

	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	raw = append(raw, '\n')

	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}

	return path, nil
}

func cliConfigPath(explicit string) (string, error) {
	if path := strings.TrimSpace(explicit); path != "" {
		return path, nil
	}

	if path := strings.TrimSpace(os.Getenv("KBEACONCTL_CONFIG")); path != "" {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(home) == "" {
		return "", errors.New("home directory is empty")
	}

	return filepath.Join(home, ".kbeaconctl", "config.json"), nil
}

func normalizeCLIConfigKey(value string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(value))
	key = strings.ReplaceAll(key, "_", "-")

	switch key {
	case "namespace", "ns":
		return "namespace", nil
	case "service", "svc":
		return "service", nil
	case "service-port", "serviceport", "port":
		return "service-port", nil
	case "kubeconfig", "kube-config":
		return "kubeconfig", nil
	case "context", "ctx":
		return "context", nil
	case "server", "url", "agent-url":
		return "server", nil
	default:
		return "", fmt.Errorf("unsupported config key %q; supported keys: %s", value, supportedCLIConfigKeys())
	}
}

func supportedCLIConfigKeys() string {
	keys := append([]string(nil), cliConfigKeys...)
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func setCLIConfigValue(cfg *cliConfig, key, value string) {
	switch key {
	case "namespace":
		cfg.Namespace = value
	case "service":
		cfg.Service = value
	case "service-port":
		cfg.ServicePort = value
	case "kubeconfig":
		cfg.Kubeconfig = value
	case "context":
		cfg.Context = value
	case "server":
		cfg.Server = value
	}
}

func unsetCLIConfigValue(cfg *cliConfig, key string) {
	switch key {
	case "namespace":
		cfg.Namespace = ""
	case "service":
		cfg.Service = ""
	case "service-port":
		cfg.ServicePort = ""
	case "kubeconfig":
		cfg.Kubeconfig = ""
	case "context":
		cfg.Context = ""
	case "server":
		cfg.Server = ""
	}
}

func cliConfigValue(cfg cliConfig, key string) string {
	switch key {
	case "namespace":
		return cfg.Namespace
	case "service":
		return cfg.Service
	case "service-port":
		return cfg.ServicePort
	case "kubeconfig":
		return cfg.Kubeconfig
	case "context":
		return cfg.Context
	case "server":
		return cfg.Server
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
