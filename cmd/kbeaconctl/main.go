package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
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
	case "snapshot":
		return runSnapshot(ctx, client, baseURL, rest[1:], stdout, stderr)
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
	if len(args) > 0 && args[0] == "report" {
		return runImpactReport(ctx, client, baseURL, args[1:], stdout, stderr)
	}

	fs := flag.NewFlagSet("impact", flag.ContinueOnError)
	fs.SetOutput(stderr)

	format := fs.String("format", "json", "Output format: json or report")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	rest := fs.Args()
	if len(rest) != 2 {
		fmt.Fprintln(stderr, "usage: kbeaconctl impact [--format json|report] <namespace> <secret-name>")
		fmt.Fprintln(stderr, "       kbeaconctl impact report <namespace> <secret-name>")
		return 2
	}

	endpoint := "/api/v1/secrets/" + url.PathEscape(rest[0]) + "/" + url.PathEscape(rest[1]) + "/impact"

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		return requestJSON(ctx, client, baseURL, endpoint, nil, stdout, stderr)
	case "report", "text":
		return requestImpactReport(ctx, client, baseURL, endpoint, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unsupported impact format %q; expected json or report\n", *format)
		return 2
	}
}

func runImpactReport(ctx context.Context, client *http.Client, baseURL *url.URL, args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: kbeaconctl impact report <namespace> <secret-name>")
		return 2
	}

	endpoint := "/api/v1/secrets/" + url.PathEscape(args[0]) + "/" + url.PathEscape(args[1]) + "/impact"
	return requestImpactReport(ctx, client, baseURL, endpoint, stdout, stderr)
}

func requestImpactReport(ctx context.Context, client *http.Client, baseURL *url.URL, endpoint string, stdout, stderr io.Writer) int {
	body, requestURL, status, err := requestBody(ctx, client, baseURL, endpoint, nil)
	if err != nil {
		fmt.Fprintf(stderr, "request %s failed: %v\n", requestURL, err)
		return 1
	}

	if status < 200 || status >= 300 {
		fmt.Fprintf(stderr, "request %s returned HTTP %d\n", requestURL, status)
		if len(body) > 0 {
			fmt.Fprintln(stderr, strings.TrimRight(string(body), "\n"))
		}
		return 1
	}

	var envelope impactEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		fmt.Fprintf(stderr, "decode Secret impact response: %v\n", err)
		return 1
	}

	renderSecretImpactReport(stdout, envelope)
	return 0
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

func runSnapshot(ctx context.Context, client *http.Client, baseURL *url.URL, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "export" {
		fmt.Fprintln(stderr, "usage: kbeaconctl snapshot export [--output FILE] [--include LIST] [--pretty=false]")
		return 2
	}

	fs := flag.NewFlagSet("snapshot export", flag.ContinueOnError)
	fs.SetOutput(stderr)

	output := fs.String("output", "-", "Output file path, or - for stdout")
	include := fs.String("include", "config,secrets,workloads,dependency-map", "Comma-separated resources: config,secrets,workloads,dependency-map")
	pretty := fs.Bool("pretty", true, "Pretty-print JSON output")

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: kbeaconctl snapshot export [--output FILE] [--include LIST] [--pretty=false]")
		return 2
	}

	return requestSnapshotExport(ctx, client, baseURL, *include, *output, *pretty, stdout, stderr)
}

type snapshotExport struct {
	APIVersion  string                     `json:"apiVersion"`
	Kind        string                     `json:"kind"`
	GeneratedAt string                     `json:"generatedAt"`
	Server      string                     `json:"server"`
	Resources   map[string]json.RawMessage `json:"resources"`
}

type snapshotResourceRequest struct {
	Name     string
	Endpoint string
}

func requestSnapshotExport(
	ctx context.Context,
	client *http.Client,
	baseURL *url.URL,
	include string,
	output string,
	pretty bool,
	stdout io.Writer,
	stderr io.Writer,
) int {
	requests, err := parseSnapshotResources(include)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --include value: %v\n", err)
		return 2
	}

	resources := make(map[string]json.RawMessage, len(requests))

	for _, item := range requests {
		body, requestURL, status, err := requestBody(ctx, client, baseURL, item.Endpoint, nil)
		if err != nil {
			fmt.Fprintf(stderr, "request %s failed: %v\n", requestURL, err)
			return 1
		}

		if status < 200 || status >= 300 {
			fmt.Fprintf(stderr, "request %s returned HTTP %d\n", requestURL, status)
			if len(body) > 0 {
				fmt.Fprintln(stderr, strings.TrimRight(string(body), "\n"))
			}
			return 1
		}

		if !json.Valid(body) {
			fmt.Fprintf(stderr, "request %s returned invalid JSON\n", requestURL)
			return 1
		}

		resources[item.Name] = append(json.RawMessage(nil), body...)
	}

	snapshot := snapshotExport{
		APIVersion:  "kbeacon.io/v1",
		Kind:        "KBeaconSnapshot",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Server:      baseURL.String(),
		Resources:   resources,
	}

	var encoded []byte
	if pretty {
		encoded, err = json.MarshalIndent(snapshot, "", "  ")
	} else {
		encoded, err = json.Marshal(snapshot)
	}
	if err != nil {
		fmt.Fprintf(stderr, "encode snapshot: %v\n", err)
		return 1
	}

	encoded = append(encoded, '\n')

	output = strings.TrimSpace(output)
	if output == "" || output == "-" {
		_, _ = stdout.Write(encoded)
		return 0
	}

	if err := os.WriteFile(output, encoded, 0o600); err != nil {
		fmt.Fprintf(stderr, "write snapshot %s: %v\n", output, err)
		return 1
	}

	fmt.Fprintf(stdout, "wrote snapshot %s\n", output)
	return 0
}

func parseSnapshotResources(value string) ([]snapshotResourceRequest, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "config,secrets,workloads,dependency-map"
	}

	parts := strings.Split(value, ",")
	out := make([]snapshotResourceRequest, 0, len(parts))
	seen := map[string]struct{}{}

	for _, part := range parts {
		item, ok := canonicalSnapshotResource(part)
		if !ok {
			return nil, fmt.Errorf("unsupported resource %q", strings.TrimSpace(part))
		}

		if _, duplicate := seen[item.Name]; duplicate {
			continue
		}

		seen[item.Name] = struct{}{}
		out = append(out, item)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no resources selected")
	}

	return out, nil
}

func canonicalSnapshotResource(value string) (snapshotResourceRequest, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "config":
		return snapshotResourceRequest{Name: "config", Endpoint: "/api/v1/config"}, true
	case "secret", "secrets":
		return snapshotResourceRequest{Name: "secrets", Endpoint: "/api/v1/secrets"}, true
	case "workload", "workloads":
		return snapshotResourceRequest{Name: "workloads", Endpoint: "/api/v1/workloads"}, true
	case "dependency-map", "dependencymap", "dependencies", "dependency", "map":
		return snapshotResourceRequest{Name: "dependencyMap", Endpoint: "/api/v1/dependency-map"}, true
	default:
		return snapshotResourceRequest{}, false
	}
}

func requestJSON(ctx context.Context, client *http.Client, baseURL *url.URL, endpoint string, query url.Values, stdout, stderr io.Writer) int {
	body, requestURL, status, err := requestBody(ctx, client, baseURL, endpoint, query)
	if err != nil {
		fmt.Fprintf(stderr, "request %s failed: %v\n", requestURL, err)
		return 1
	}

	if status < 200 || status >= 300 {
		fmt.Fprintf(stderr, "request %s returned HTTP %d\n", requestURL, status)
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

func requestBody(ctx context.Context, client *http.Client, baseURL *url.URL, endpoint string, query url.Values) ([]byte, string, int, error) {
	u := *baseURL
	u.Path = strings.TrimRight(u.Path, "/") + endpoint
	if query != nil {
		u.RawQuery = query.Encode()
	}

	requestURL := u.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, requestURL, 0, fmt.Errorf("build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, requestURL, 0, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, requestURL, resp.StatusCode, fmt.Errorf("read response: %w", readErr)
	}

	return body, requestURL, resp.StatusCode, nil
}

type impactEnvelope struct {
	APIVersion  string     `json:"apiVersion"`
	Cluster     string     `json:"cluster"`
	GeneratedAt string     `json:"generatedAt"`
	Data        impactData `json:"data"`
}

type impactData struct {
	Secret            impactSecretSummary     `json:"secret"`
	Summary           impactSummary           `json:"summary"`
	AffectedTeams     []impactAffectedTeam    `json:"affectedTeams"`
	AffectedWorkloads []impactWorkloadSummary `json:"affectedWorkloads"`
	Edges             []impactEdge            `json:"edges"`
}

type impactSecretSummary struct {
	Ref                      impactSecretRef `json:"ref"`
	Exists                   bool            `json:"exists"`
	Type                     string          `json:"type"`
	OwnerTeam                string          `json:"ownerTeam"`
	Criticality              string          `json:"criticality"`
	AffectedWorkloadCount    int             `json:"affectedWorkloadCount"`
	AffectedTeamCount        int             `json:"affectedTeamCount"`
	AffectedNamespaceCount   int             `json:"affectedNamespaceCount"`
	UnresolvedReferenceCount int             `json:"unresolvedReferenceCount"`
	ImpactScore              float64         `json:"impactScore"`
}

type impactSummary struct {
	AffectedWorkloadCount    int            `json:"affectedWorkloadCount"`
	AffectedTeamCount        int            `json:"affectedTeamCount"`
	AffectedNamespaceCount   int            `json:"affectedNamespaceCount"`
	UnresolvedReferenceCount int            `json:"unresolvedReferenceCount"`
	DiscoveryModes           map[string]int `json:"discoveryModes"`
}

type impactAffectedTeam struct {
	OwnerTeam     string `json:"ownerTeam"`
	WorkloadCount int    `json:"workloadCount"`
}

type impactWorkloadSummary struct {
	Ref             impactWorkloadRef `json:"ref"`
	OwnerTeam       string            `json:"ownerTeam"`
	Service         string            `json:"service"`
	Environment     string            `json:"environment"`
	Criticality     string            `json:"criticality"`
	DiscoveryMode   string            `json:"discoveryMode"`
	DependencyCount int               `json:"dependencyCount"`
	UnresolvedCount int               `json:"unresolvedCount"`
}

type impactEdge struct {
	Workload      impactWorkloadRef `json:"workload"`
	Secret        impactSecretRef   `json:"secret"`
	DiscoveryMode string            `json:"discoveryMode"`
	Sources       []impactSource    `json:"sources"`
	Optional      bool              `json:"optional"`
	Resolved      bool              `json:"resolved"`
	OwnerTeam     string            `json:"ownerTeam"`
	Criticality   string            `json:"criticality"`
}

type impactSecretRef struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Key       string `json:"key"`
}

type impactWorkloadRef struct {
	Cluster    string `json:"cluster"`
	Namespace  string `json:"namespace"`
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
}

type impactSource struct {
	Type          string `json:"type"`
	Path          string `json:"path"`
	Container     string `json:"container"`
	InitContainer string `json:"initContainer"`
	Ephemeral     bool   `json:"ephemeralContainer"`
	Volume        string `json:"volume"`
	EnvVar        string `json:"envVar"`
	Annotation    string `json:"annotation"`
	ResourceField string `json:"resourceField"`
}

func renderSecretImpactReport(w io.Writer, envelope impactEnvelope) {
	data := envelope.Data
	secret := data.Secret.Ref

	fmt.Fprintln(w, "KBeacon Secret Impact Report")
	fmt.Fprintf(w, "Cluster: %s\n", valueOr(envelope.Cluster, "unknown"))
	fmt.Fprintf(w, "Generated at: %s\n", valueOr(envelope.GeneratedAt, "unknown"))
	fmt.Fprintf(w, "Secret: %s/%s\n", valueOr(secret.Namespace, "unknown"), valueOr(secret.Name, "unknown"))
	fmt.Fprintf(w, "Exists: %s\n", yesNo(data.Secret.Exists))

	if data.Secret.Type != "" {
		fmt.Fprintf(w, "Type: %s\n", data.Secret.Type)
	}
	if data.Secret.OwnerTeam != "" {
		fmt.Fprintf(w, "Owner team: %s\n", data.Secret.OwnerTeam)
	}
	if data.Secret.Criticality != "" {
		fmt.Fprintf(w, "Criticality: %s\n", data.Secret.Criticality)
	}

	fmt.Fprintf(w, "Impact score: %.2f\n", data.Secret.ImpactScore)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  Affected workloads: %d\n", data.Summary.AffectedWorkloadCount)
	fmt.Fprintf(w, "  Affected teams: %d\n", data.Summary.AffectedTeamCount)
	fmt.Fprintf(w, "  Affected namespaces: %d\n", data.Summary.AffectedNamespaceCount)
	fmt.Fprintf(w, "  Unresolved references: %d\n", data.Summary.UnresolvedReferenceCount)
	fmt.Fprintf(w, "  Discovery modes: %s\n", formatDiscoveryModes(data.Summary.DiscoveryModes))

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Affected teams:")
	if len(data.AffectedTeams) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, team := range data.AffectedTeams {
			fmt.Fprintf(w, "  - %s: %d workload(s)\n", valueOr(team.OwnerTeam, "unknown"), team.WorkloadCount)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Affected workloads:")
	if len(data.AffectedWorkloads) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, workload := range data.AffectedWorkloads {
			ref := workload.Ref
			details := []string{}
			if workload.OwnerTeam != "" {
				details = append(details, "ownerTeam="+workload.OwnerTeam)
			}
			if workload.Criticality != "" {
				details = append(details, "criticality="+workload.Criticality)
			}
			if workload.DiscoveryMode != "" {
				details = append(details, "mode="+workload.DiscoveryMode)
			}
			details = append(details, fmt.Sprintf("dependencies=%d", workload.DependencyCount))
			details = append(details, fmt.Sprintf("unresolved=%d", workload.UnresolvedCount))

			fmt.Fprintf(
				w,
				"  - %s %s/%s (%s)\n",
				valueOr(ref.Kind, "Workload"),
				valueOr(ref.Namespace, "unknown"),
				valueOr(ref.Name, "unknown"),
				strings.Join(details, ", "),
			)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Dependency edges:")
	if len(data.Edges) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, edge := range data.Edges {
			fmt.Fprintf(
				w,
				"  - %s %s/%s -> Secret %s/%s mode=%s resolved=%s optional=%s\n",
				valueOr(edge.Workload.Kind, "Workload"),
				valueOr(edge.Workload.Namespace, "unknown"),
				valueOr(edge.Workload.Name, "unknown"),
				valueOr(edge.Secret.Namespace, "unknown"),
				valueOr(edge.Secret.Name, "unknown"),
				valueOr(edge.DiscoveryMode, "unknown"),
				yesNo(edge.Resolved),
				yesNo(edge.Optional),
			)
			fmt.Fprintf(w, "    sources: %s\n", sourceTypes(edge.Sources))
		}
	}
}

func formatDiscoveryModes(modes map[string]int) string {
	if len(modes) == 0 {
		return "(none)"
	}

	keys := make([]string, 0, len(modes))
	for key := range modes {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, modes[key]))
	}

	return strings.Join(parts, ", ")
}

func sourceTypes(sources []impactSource) string {
	if len(sources) == 0 {
		return "(none)"
	}

	seen := map[string]struct{}{}
	out := []string{}

	for _, source := range sources {
		sourceType := strings.TrimSpace(source.Type)
		if sourceType == "" {
			continue
		}
		if _, ok := seen[sourceType]; ok {
			continue
		}
		seen[sourceType] = struct{}{}
		out = append(out, sourceType)
	}

	if len(out) == 0 {
		return "(none)"
	}

	sort.Strings(out)
	return strings.Join(out, ", ")
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
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
	fmt.Fprintln(w, "  impact report <namespace> <secret-name>      Print a human-readable Secret impact report")
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
