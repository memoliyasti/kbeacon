package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	version     = "dev"
	commit      = "unknown"
	programName = "kbeaconctl"
)

func main() {
	if name := strings.TrimSpace(filepath.Base(os.Args[0])); name != "" && name != "." {
		programName = name
	}

	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func commandUsageName() string {
	name := strings.TrimSpace(programName)
	if name == "" {
		return "kbeaconctl"
	}

	if slash := strings.LastIndexAny(name, `/\`); slash >= 0 {
		name = name[slash+1:]
	}

	name = strings.TrimSuffix(name, ".exe")

	switch {
	case name == "kbeaconctl" || strings.HasPrefix(name, "kbeaconctl_") || strings.HasPrefix(name, "kbeaconctl-"):
		return "kbeaconctl"
	case name == "kbeacon" || strings.HasPrefix(name, "kbeacon_") || strings.HasPrefix(name, "kbeacon-"):
		return "kbeacon"
	default:
		return name
	}
}

func usage(command string) string {
	return fmt.Sprintf("usage: %s %s", commandUsageName(), command)
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("kbeaconctl", flag.ContinueOnError)
	fs.SetOutput(stderr)

	serverRaw := fs.String("server", "", "Optional direct KBeacon Agent base URL; empty uses the Kubernetes service proxy")
	namespaceRaw := fs.String("namespace", "", "KBeacon Agent namespace for Kubernetes service proxy")
	namespaceAlias := fs.String("n", "", "Alias for --namespace")
	serviceRaw := fs.String("service", "", "KBeacon Agent Service name for Kubernetes service proxy")
	servicePortRaw := fs.String("service-port", "", "KBeacon Agent Service port name or number for Kubernetes service proxy")
	kubeconfigRaw := fs.String("kubeconfig", "", "Path to kubeconfig file")
	contextRaw := fs.String("context", "", "Kubeconfig context override")
	configFileRaw := fs.String("config-file", getenv("KBEACONCTL_CONFIG", ""), "Path to kbeaconctl config file")
	timeoutRaw := fs.String("timeout", getenv("KBEACONCTL_TIMEOUT", "10s"), "Request timeout, for example 5s or 30s")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if strings.TrimSpace(*namespaceAlias) != "" {
		*namespaceRaw = *namespaceAlias
	}

	rest := fs.Args()
	if len(rest) == 0 {
		printUsage(stderr)
		return 2
	}

	command := rest[0]

	if command == "version" {
		fmt.Fprintf(stdout, "%s version=%s commit=%s\n", commandUsageName(), version, commit)
		return 0
	}

	if command == "help" || command == "-h" || command == "--help" {
		printUsage(stdout)
		return 0
	}

	if command == "snapshot" && len(rest) > 1 && rest[1] == "diff" {
		return runSnapshotDiff(rest[2:], stdout, stderr)
	}

	if command == "config" {
		return runConfig(*configFileRaw, rest[1:], stdout, stderr)
	}

	switch command {
	case "health", "ready", "api", "get", "impact", "dependencies", "snapshot", "raw":
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", command)
		printUsage(stderr)
		return 2
	}

	timeout, err := time.ParseDuration(*timeoutRaw)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --timeout value %q: %v\n", *timeoutRaw, err)
		return 2
	}

	storedConfig, _, err := loadCLIConfig(*configFileRaw)
	if err != nil {
		fmt.Fprintf(stderr, "load CLI config: %v\n", err)
		return 2
	}

	agent, err := newAgentClient(clientOptions{
		Server: firstNonEmpty(
			*serverRaw,
			getenv("KBEACONCTL_SERVER", ""),
			storedConfig.Server,
		),
		Namespace: firstNonEmpty(
			*namespaceRaw,
			getenv("KBEACONCTL_NAMESPACE", getenv("KBEACON_NAMESPACE", "")),
			storedConfig.Namespace,
			defaultKBeaconNamespace,
		),
		Service: firstNonEmpty(
			*serviceRaw,
			getenv("KBEACONCTL_SERVICE", ""),
			storedConfig.Service,
			defaultKBeaconService,
		),
		ServicePort: firstNonEmpty(
			*servicePortRaw,
			getenv("KBEACONCTL_SERVICE_PORT", ""),
			storedConfig.ServicePort,
			defaultKBeaconServicePort,
		),
		Kubeconfig: firstNonEmpty(
			*kubeconfigRaw,
			getenv("KBEACONCTL_KUBECONFIG", ""),
			storedConfig.Kubeconfig,
		),
		Context: firstNonEmpty(
			*contextRaw,
			getenv("KBEACONCTL_CONTEXT", ""),
			storedConfig.Context,
		),
		Timeout: timeout,
	})
	if err != nil {
		fmt.Fprintf(stderr, "create KBeacon client: %v\n", err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch command {
	case "health":
		return requestJSON(ctx, agent, "/healthz", nil, stdout, stderr)
	case "ready":
		return requestJSON(ctx, agent, "/readyz", nil, stdout, stderr)
	case "api":
		return requestJSON(ctx, agent, "/api/v1", nil, stdout, stderr)
	case "get":
		return runGet(ctx, agent, rest[1:], stdout, stderr)
	case "impact":
		return runImpact(ctx, agent, rest[1:], stdout, stderr)
	case "dependencies":
		return runDependencies(ctx, agent, rest[1:], stdout, stderr)
	case "snapshot":
		return runSnapshot(ctx, agent, rest[1:], stdout, stderr)
	case "raw":
		return runRaw(ctx, agent, rest[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", command)
		printUsage(stderr)
		return 2
	}
}

func newAgentClient(opts clientOptions) (agentClient, error) {
	if strings.TrimSpace(opts.Server) != "" {
		return newDirectAgentClient(opts.Server, opts.Timeout)
	}

	return newKubeServiceProxyClient(opts)
}

func runGet(ctx context.Context, agent agentClient, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage("get <secrets|workloads|dependency-map|config> [filters]"))
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
		return requestJSON(ctx, agent, "/api/v1/secrets", q, stdout, stderr)

	case "workloads", "workload":
		addNonEmpty(q, "namespace", *namespace)
		addNonEmpty(q, "ownerTeam", *ownerTeam)
		addNonEmpty(q, "criticality", *criticality)
		addNonEmpty(q, "workloadKind", *workloadKind)
		addNonEmpty(q, "workloadName", *workloadName)
		addNonEmpty(q, "discoveryMode", *discoveryMode)
		addNonEmpty(q, "limit", *limit)
		addNonEmpty(q, "offset", *offset)
		return requestJSON(ctx, agent, "/api/v1/workloads", q, stdout, stderr)

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
		return requestJSON(ctx, agent, "/api/v1/dependency-map", q, stdout, stderr)

	case "config":
		return requestJSON(ctx, agent, "/api/v1/config", nil, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "unknown get resource %q\n", resource)
		fmt.Fprintln(stderr, "supported resources: secrets, workloads, dependency-map, config")
		return 2
	}
}

func runImpact(ctx context.Context, agent agentClient, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "report" {
		return runImpactReport(ctx, agent, args[1:], stdout, stderr)
	}

	fs := flag.NewFlagSet("impact", flag.ContinueOnError)
	fs.SetOutput(stderr)

	format := fs.String("format", "json", "Output format: json or report")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	rest := fs.Args()
	if len(rest) != 2 {
		fmt.Fprintln(stderr, usage("impact [--format json|report] <namespace> <secret-name>"))
		fmt.Fprintf(stderr, "       %s impact report <namespace> <secret-name>\n", commandUsageName())
		return 2
	}

	endpoint := "/api/v1/secrets/" + url.PathEscape(rest[0]) + "/" + url.PathEscape(rest[1]) + "/impact"

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		return requestJSON(ctx, agent, endpoint, nil, stdout, stderr)
	case "report", "text":
		return requestImpactReport(ctx, agent, endpoint, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unsupported impact format %q; expected json or report\n", *format)
		return 2
	}
}

func runImpactReport(ctx context.Context, agent agentClient, args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, usage("impact report <namespace> <secret-name>"))
		return 2
	}

	endpoint := "/api/v1/secrets/" + url.PathEscape(args[0]) + "/" + url.PathEscape(args[1]) + "/impact"
	return requestImpactReport(ctx, agent, endpoint, stdout, stderr)
}

func requestImpactReport(ctx context.Context, agent agentClient, endpoint string, stdout, stderr io.Writer) int {
	body, requestURL, status, err := requestBody(ctx, agent, endpoint, nil)
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

func runDependencies(ctx context.Context, agent agentClient, args []string, stdout, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintln(stderr, usage("dependencies <namespace> <workload-kind> <workload-name>"))
		return 2
	}

	endpoint := "/api/v1/workloads/" + url.PathEscape(args[0]) + "/" + url.PathEscape(args[1]) + "/" + url.PathEscape(args[2]) + "/dependencies"
	return requestJSON(ctx, agent, endpoint, nil, stdout, stderr)
}

func runRaw(ctx context.Context, agent agentClient, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, usage("raw <api-path>"))
		return 2
	}

	endpoint := args[0]
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}

	return requestJSON(ctx, agent, endpoint, nil, stdout, stderr)
}

func runSnapshot(ctx context.Context, agent agentClient, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage("snapshot <export|diff> [args]"))
		return 2
	}

	if args[0] == "diff" {
		return runSnapshotDiff(args[1:], stdout, stderr)
	}

	if args[0] != "export" {
		fmt.Fprintln(stderr, usage("snapshot <export|diff> [args]"))
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
		fmt.Fprintln(stderr, usage("snapshot export [--output FILE] [--include LIST] [--pretty=false]"))
		return 2
	}

	return requestSnapshotExport(ctx, agent, *include, *output, *pretty, stdout, stderr)
}

type snapshotExport struct {
	APIVersion  string                     `json:"apiVersion"`
	Kind        string                     `json:"kind"`
	GeneratedAt string                     `json:"generatedAt"`
	Cluster     string                     `json:"cluster,omitempty"`
	Server      string                     `json:"server"`
	Resources   map[string]json.RawMessage `json:"resources"`
}

func clusterFromSnapshotResources(resources map[string]json.RawMessage) string {
	raw, ok := resources["config"]
	if !ok {
		return ""
	}

	var envelope struct {
		Cluster string `json:"cluster"`
		Data    struct {
			Cluster struct {
				Name string `json:"name"`
			} `json:"cluster"`
		} `json:"data"`
	}

	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ""
	}

	if cluster := strings.TrimSpace(envelope.Cluster); cluster != "" {
		return cluster
	}

	return strings.TrimSpace(envelope.Data.Cluster.Name)
}

type snapshotResourceRequest struct {
	Name     string
	Endpoint string
}

func requestSnapshotExport(
	ctx context.Context,
	agent agentClient,
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
		body, requestURL, status, err := requestBody(ctx, agent, item.Endpoint, nil)
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
		Cluster:     clusterFromSnapshotResources(resources),
		Server:      agent.Description(),
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

func requestJSON(ctx context.Context, agent agentClient, endpoint string, query url.Values, stdout, stderr io.Writer) int {
	body, requestURL, status, err := requestBody(ctx, agent, endpoint, query)
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

func requestBody(ctx context.Context, agent agentClient, endpoint string, query url.Values) ([]byte, string, int, error) {
	return agent.Get(ctx, endpoint, query)
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

func addNonEmpty(values url.Values, key, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		values.Set(key, value)
	}
}

func printUsage(w io.Writer) {
	cmd := commandUsageName()

	fmt.Fprintf(w, "usage: %s [global flags] <command> [args]\n", cmd)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Default connection:")
	fmt.Fprintln(w, "  Uses the current kubeconfig context and the Kubernetes service proxy.")
	fmt.Fprintln(w, "  No kubectl port-forward is required.")
	fmt.Fprintf(w, "  Default Agent Service: %s/%s:%s\n", defaultKBeaconNamespace, defaultKBeaconService, defaultKBeaconServicePort)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Global flags must be placed before the command:")
	fmt.Fprintln(w, "  --namespace, -n NAME       KBeacon Agent namespace")
	fmt.Fprintln(w, "  --service NAME             KBeacon Agent Service name")
	fmt.Fprintln(w, "  --service-port NAME        KBeacon Agent Service port name or number")
	fmt.Fprintln(w, "  --kubeconfig FILE          kubeconfig path")
	fmt.Fprintln(w, "  --context NAME             kubeconfig context")
	fmt.Fprintln(w, "  --config-file FILE         kbeaconctl config file")
	fmt.Fprintln(w, "  --server URL               direct Agent URL override; disables Kubernetes proxy mode")
	fmt.Fprintln(w, "  --timeout DURATION         request timeout")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  version                                      Print CLI version")
	fmt.Fprintln(w, "  health                                       Get Agent liveness")
	fmt.Fprintln(w, "  ready                                        Get Agent readiness")
	fmt.Fprintln(w, "  api                                          Get API discovery")
	fmt.Fprintln(w, "  get secrets [filters]                       List Secrets")
	fmt.Fprintln(w, "  get workloads [filters]                     List workloads")
	fmt.Fprintln(w, "  get dependency-map [filters]                Get dependency map")
	fmt.Fprintln(w, "  get config                                  Get graph summary")
	fmt.Fprintln(w, "  impact <namespace> <secret-name>            Get Secret impact JSON")
	fmt.Fprintln(w, "  impact report <namespace> <secret-name>     Print a human-readable Secret impact report")
	fmt.Fprintln(w, "  dependencies <ns> <kind> <name>             Get workload dependencies")
	fmt.Fprintln(w, "  snapshot export [flags]                     Export a KBeacon snapshot")
	fmt.Fprintln(w, "  snapshot diff [flags] OLD NEW               Diff two snapshot files")
	fmt.Fprintln(w, "  config view|get|set|unset|path|reset        Manage persistent CLI defaults")
	fmt.Fprintln(w, "  raw <api-path>                              Request an arbitrary Agent API path")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintf(w, "  %s config set namespace kbeacon-system\n", cmd)
	fmt.Fprintf(w, "  %s ready\n", cmd)
	fmt.Fprintf(w, "  %s get secrets --namespace payments\n", cmd)
	fmt.Fprintf(w, "  %s impact report payments payments-db\n", cmd)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Environment:")
	fmt.Fprintln(w, "  KBEACONCTL_CONFIG         kbeaconctl config file")
	fmt.Fprintln(w, "  KBEACONCTL_NAMESPACE      default Agent namespace")
	fmt.Fprintln(w, "  KBEACON_NAMESPACE         default Agent namespace fallback")
	fmt.Fprintln(w, "  KBEACONCTL_SERVICE        default Agent Service name")
	fmt.Fprintln(w, "  KBEACONCTL_SERVICE_PORT   default Agent Service port")
	fmt.Fprintln(w, "  KBEACONCTL_KUBECONFIG    kubeconfig path override")
	fmt.Fprintln(w, "  KBEACONCTL_CONTEXT        kubeconfig context override")
	fmt.Fprintln(w, "  KBEACONCTL_SERVER         direct Agent URL override")
	fmt.Fprintln(w, "  KBEACONCTL_TIMEOUT        request timeout")
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
