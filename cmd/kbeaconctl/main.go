package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("kbeaconctl", flag.ContinueOnError)
	fs.SetOutput(stderr)

	serverRaw := fs.String("server", getenv("KBEACONCTL_SERVER", "http://127.0.0.1:8081"), "KBeacon Agent base URL")
	timeoutRaw := fs.String("timeout", getenv("KBEACONCTL_TIMEOUT", "10s"), "HTTP timeout, for example 5s or 30s")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	rest := fs.Args()
	if len(rest) == 0 {
		printUsage(stderr)
		return 2
	}

	command := rest[0]

	if command == "version" {
		fmt.Fprintf(stdout, "kbeaconctl version=%s commit=%s\n", version, commit)
		return 0
	}

	timeout, err := time.ParseDuration(*timeoutRaw)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --timeout value %q: %v\n", *timeoutRaw, err)
		return 2
	}

	baseURL, err := normalizeBaseURL(*serverRaw)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --server value %q: %v\n", *serverRaw, err)
		return 2
	}

	client := &http.Client{Timeout: timeout}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch command {
	case "health":
		return requestJSON(ctx, client, baseURL, "/healthz", nil, stdout, stderr)
	case "ready":
		return requestJSON(ctx, client, baseURL, "/readyz", nil, stdout, stderr)
	case "api":
		return requestJSON(ctx, client, baseURL, "/api/v1", nil, stdout, stderr)
	case "get":
		return runGet(ctx, client, baseURL, rest[1:], stdout, stderr)
	case "impact":
		return runImpact(ctx, client, baseURL, rest[1:], stdout, stderr)
	case "dependencies":
		return runDependencies(ctx, client, baseURL, rest[1:], stdout, stderr)
	case "raw":
		return runRaw(ctx, client, baseURL, rest[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", command)
		printUsage(stderr)
		return 2
	}
}

func runGet(ctx context.Context, client *http.Client, baseURL *url.URL, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: kbeaconctl get <secrets|workloads|dependency-map|config> [filters]")
		return 2
	}

	resource := strings.ToLower(strings.TrimSpace(args[0]))

	fs := flag.NewFlagSet("get "+resource, flag.ContinueOnError)
	fs.SetOutput(stderr)

	namespace := fs.String("namespace", "", "Namespace filter")
	ownerTeam := fs.String("owner-team", "", "Owner team filter")
	criticality := fs.String("criticality", "", "Criticality filter")
	exists := fs.String("exists", "", "Secret existence filter: true or false")
	resolved := fs.String("resolved", "", "Dependency resolution filter: true or false")
	secretName := fs.String("secret-name", "", "Secret name filter")
	secretNamespace := fs.String("secret-namespace", "", "Secret namespace filter")
	workloadName := fs.String("workload-name", "", "Workload name filter")
	workloadKind := fs.String("workload-kind", "", "Workload kind filter")
	workloadNamespace := fs.String("workload-namespace", "", "Workload namespace filter")
	discoveryMode := fs.String("discovery-mode", "", "Discovery mode filter")
	limit := fs.String("limit", "", "Pagination limit")
	offset := fs.String("offset", "", "Pagination offset")

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	q := url.Values{}

	switch resource {
	case "secrets", "secret":
		addNonEmpty(q, "namespace", *namespace)
		addNonEmpty(q, "ownerTeam", *ownerTeam)
		addNonEmpty(q, "criticality", *criticality)
		addNonEmpty(q, "exists", *exists)
		addNonEmpty(q, "secretName", *secretName)
		addNonEmpty(q, "limit", *limit)
		addNonEmpty(q, "offset", *offset)
		return requestJSON(ctx, client, baseURL, "/api/v1/secrets", q, stdout, stderr)

	case "workloads", "workload":
		addNonEmpty(q, "namespace", *namespace)
		addNonEmpty(q, "ownerTeam", *ownerTeam)
		addNonEmpty(q, "criticality", *criticality)
		addNonEmpty(q, "workloadKind", *workloadKind)
		addNonEmpty(q, "workloadName", *workloadName)
		addNonEmpty(q, "discoveryMode", *discoveryMode)
		addNonEmpty(q, "limit", *limit)
		addNonEmpty(q, "offset", *offset)
		return requestJSON(ctx, client, baseURL, "/api/v1/workloads", q, stdout, stderr)

	case "dependency-map", "dependency", "dependencies", "map":
		addNonEmpty(q, "namespace", *namespace)
		addNonEmpty(q, "workloadNamespace", *workloadNamespace)
		addNonEmpty(q, "secretNamespace", *secretNamespace)
		addNonEmpty(q, "workloadKind", *workloadKind)
		addNonEmpty(q, "workloadName", *workloadName)
		addNonEmpty(q, "secretName", *secretName)
		addNonEmpty(q, "ownerTeam", *ownerTeam)
		addNonEmpty(q, "criticality", *criticality)
		addNonEmpty(q, "resolved", *resolved)
		addNonEmpty(q, "discoveryMode", *discoveryMode)
		addNonEmpty(q, "limit", *limit)
		addNonEmpty(q, "offset", *offset)
		return requestJSON(ctx, client, baseURL, "/api/v1/dependency-map", q, stdout, stderr)

	case "config":
		return requestJSON(ctx, client, baseURL, "/api/v1/config", nil, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "unknown get resource %q\n", resource)
		fmt.Fprintln(stderr, "supported resources: secrets, workloads, dependency-map, config")
		return 2
	}
}

func runImpact(ctx context.Context, client *http.Client, baseURL *url.URL, args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: kbeaconctl impact <namespace> <secret-name>")
		return 2
	}

	endpoint := "/api/v1/secrets/" + url.PathEscape(args[0]) + "/" + url.PathEscape(args[1]) + "/impact"
	return requestJSON(ctx, client, baseURL, endpoint, nil, stdout, stderr)
}

func runDependencies(ctx context.Context, client *http.Client, baseURL *url.URL, args []string, stdout, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintln(stderr, "usage: kbeaconctl dependencies <namespace> <workload-kind> <workload-name>")
		return 2
	}

	endpoint := "/api/v1/workloads/" + url.PathEscape(args[0]) + "/" + url.PathEscape(args[1]) + "/" + url.PathEscape(args[2]) + "/dependencies"
	return requestJSON(ctx, client, baseURL, endpoint, nil, stdout, stderr)
}

func runRaw(ctx context.Context, client *http.Client, baseURL *url.URL, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: kbeaconctl raw <api-path>")
		return 2
	}

	endpoint := args[0]
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}

	return requestJSON(ctx, client, baseURL, endpoint, nil, stdout, stderr)
}

func requestJSON(ctx context.Context, client *http.Client, baseURL *url.URL, endpoint string, query url.Values, stdout, stderr io.Writer) int {
	u := *baseURL
	u.Path = strings.TrimRight(u.Path, "/") + endpoint
	if query != nil {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		fmt.Fprintf(stderr, "build request: %v\n", err)
		return 1
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "request %s failed: %v\n", u.String(), err)
		return 1
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		fmt.Fprintf(stderr, "read response: %v\n", readErr)
		return 1
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(stderr, "request %s returned HTTP %d\n", u.String(), resp.StatusCode)
		if len(body) > 0 {
			fmt.Fprintln(stderr, strings.TrimRight(string(body), "\n"))
		}
		return 1
	}

	if len(body) > 0 {
		_, _ = stdout.Write(body)
		if body[len(body)-1] != '\n' {
			fmt.Fprintln(stdout)
		}
	}

	return 0
}

func normalizeBaseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty URL")
	}

	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("URL must include scheme and host")
	}

	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""

	return u, nil
}

func addNonEmpty(values url.Values, key, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		values.Set(key, value)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: kbeaconctl [--server URL] [--timeout DURATION] <command> [args]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  version                                      Print CLI version")
	fmt.Fprintln(w, "  health                                       GET /healthz")
	fmt.Fprintln(w, "  ready                                        GET /readyz")
	fmt.Fprintln(w, "  api                                          GET /api/v1")
	fmt.Fprintln(w, "  get secrets [filters]                        List Secrets")
	fmt.Fprintln(w, "  get workloads [filters]                      List workloads")
	fmt.Fprintln(w, "  get dependency-map [filters]                 Get dependency map")
	fmt.Fprintln(w, "  get config                                   Get Agent graph summary")
	fmt.Fprintln(w, "  impact <namespace> <secret-name>             Get Secret impact JSON")
	fmt.Fprintln(w, "  dependencies <namespace> <kind> <name>       Get workload dependency JSON")
	fmt.Fprintln(w, "  raw <api-path>                               GET an arbitrary Agent API path")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "common filters:")
	fmt.Fprintln(w, "  --namespace, --owner-team, --criticality, --secret-name")
	fmt.Fprintln(w, "  --workload-namespace, --workload-kind, --workload-name")
	fmt.Fprintln(w, "  --secret-namespace, --exists, --resolved, --discovery-mode")
	fmt.Fprintln(w, "  --limit, --offset")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "environment:")
	fmt.Fprintln(w, "  KBEACONCTL_SERVER   default Agent URL")
	fmt.Fprintln(w, "  KBEACONCTL_TIMEOUT  default HTTP timeout")
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
