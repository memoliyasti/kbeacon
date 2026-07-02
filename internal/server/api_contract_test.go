package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/yaml"
)

func TestOpenAPIContract(t *testing.T) {
	doc := contractLoadOpenAPI(t)

	if got := contractString(t, doc, "openapi"); got != "3.1.0" {
		t.Fatalf("expected OpenAPI version 3.1.0, got %q", got)
	}

	info := contractObject(t, doc, "info")
	chartVersion := contractChartVersion(t)
	if got := contractString(t, info, "version"); got != chartVersion {
		t.Fatalf("OpenAPI info.version=%q must match chart version=%q", got, chartVersion)
	}

	contractRequirePaths(t, doc, []string{
		"/healthz",
		"/readyz",
		"/api/v1",
		"/api/v1/config",
		"/api/v1/secrets",
		"/api/v1/workloads",
		"/api/v1/dependency-map",
		"/api/v1/secrets/{namespace}/{name}/impact",
		"/api/v1/workloads/{namespace}/{name}/dependencies",
		"/api/v1/workloads/{namespace}/{kind}/{name}/dependencies",
	})

	contractRequireParams(t, doc, "/api/v1/secrets", []string{
		"namespace",
		"ownerTeam",
		"criticality",
		"exists",
		"secretName",
		"limit",
		"offset",
	})

	contractRequireParams(t, doc, "/api/v1/workloads", []string{
		"namespace",
		"ownerTeam",
		"criticality",
		"workloadKind",
		"workloadName",
		"discoveryMode",
		"limit",
		"offset",
	})

	contractRequireParams(t, doc, "/api/v1/dependency-map", []string{
		"namespace",
		"workloadNamespace",
		"secretNamespace",
		"workloadKind",
		"workloadName",
		"secretName",
		"ownerTeam",
		"criticality",
		"resolved",
		"discoveryMode",
		"limit",
		"offset",
	})

	contractRequireUniqueOperationIDs(t, doc)

	components := contractObject(t, doc, "components")
	schemas := contractObject(t, components, "schemas")

	contractRequireSchemaProps(t, schemas, "Envelope", []string{
		"apiVersion",
		"cluster",
		"generatedAt",
		"pagination",
		"data",
	}, nil)

	contractRequireSchemaProps(t, schemas, "Pagination", []string{
		"limit",
		"offset",
		"total",
		"returned",
		"nextOffset",
	}, nil)

	contractRequireSchemaProps(t, schemas, "DependencyEdge", []string{
		"id",
		"cluster",
		"workload",
		"secret",
		"discoveryMode",
		"sources",
		"optional",
		"resolved",
	}, []string{
		"from",
		"to",
	})

	contractRequireSchemaProps(t, schemas, "SecretImpact", []string{
		"secret",
		"summary",
		"affectedTeams",
		"affectedWorkloads",
		"edges",
	}, []string{
		"workloads",
	})

	contractRequireSchemaProps(t, schemas, "WorkloadDependencies", []string{
		"workload",
		"dependencies",
	}, []string{
		"secrets",
		"edges",
	})
}

func TestAPIExampleContracts(t *testing.T) {
	expectations := map[string]contractExampleExpectation{
		"dependency-map.json": {
			hasPagination: true,
			kind:          "dependency-map",
		},
		"secret-impact.json": {
			hasPagination: false,
			kind:          "secret-impact",
		},
		"secrets-list.json": {
			hasPagination: true,
			kind:          "secrets-list",
		},
		"workload-dependencies.json": {
			hasPagination: false,
			kind:          "workload-dependencies",
		},
		"workloads-list.json": {
			hasPagination: true,
			kind:          "workloads-list",
		},
	}

	dir := contractRepoPath("examples", "api")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read API examples directory: %v", err)
	}

	seen := map[string]bool{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		expectation, ok := expectations[entry.Name()]
		if !ok {
			t.Fatalf("unexpected API example file %s; add it to the contract test", entry.Name())
		}

		seen[entry.Name()] = true

		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}

		contractValidateEnvelope(t, entry.Name(), doc, expectation.hasPagination)
		contractValidateExampleData(t, entry.Name(), expectation.kind, doc["data"])
	}

	for name := range expectations {
		if !seen[name] {
			t.Fatalf("expected API example %s is missing", name)
		}
	}
}

func TestHandlerResponsesMatchAPIContractShapes(t *testing.T) {
	h := newRichTestServer()

	cases := []struct {
		name          string
		path          string
		hasPagination bool
		kind          string
	}{
		{
			name:          "config",
			path:          "/api/v1/config",
			hasPagination: false,
			kind:          "config",
		},
		{
			name:          "secrets",
			path:          "/api/v1/secrets?limit=1",
			hasPagination: true,
			kind:          "secrets-list",
		},
		{
			name:          "workloads",
			path:          "/api/v1/workloads?limit=1",
			hasPagination: true,
			kind:          "workloads-list",
		},
		{
			name:          "dependency-map",
			path:          "/api/v1/dependency-map?limit=2",
			hasPagination: true,
			kind:          "dependency-map",
		},
		{
			name:          "secret-impact",
			path:          "/api/v1/secrets/payments/payments-db/impact",
			hasPagination: false,
			kind:          "secret-impact",
		},
		{
			name:          "workload-dependencies",
			path:          "/api/v1/workloads/payments/Deployment/payments-api/dependencies",
			hasPagination: false,
			kind:          "workload-dependencies",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := getJSON(t, h, tc.path)
			contractValidateEnvelope(t, tc.name, out, tc.hasPagination)
			contractValidateExampleData(t, tc.name, tc.kind, out["data"])
		})
	}
}

type contractExampleExpectation struct {
	hasPagination bool
	kind          string
}

func contractValidateEnvelope(t *testing.T, name string, doc map[string]any, hasPagination bool) {
	t.Helper()

	if got := contractString(t, doc, "apiVersion"); got != "kbeacon.io/v1" {
		t.Fatalf("%s: expected apiVersion kbeacon.io/v1, got %q", name, got)
	}

	if got := contractString(t, doc, "cluster"); strings.TrimSpace(got) == "" {
		t.Fatalf("%s: expected non-empty cluster", name)
	}

	generatedAt := contractString(t, doc, "generatedAt")
	if _, err := time.Parse(time.RFC3339, generatedAt); err != nil {
		t.Fatalf("%s: generatedAt must be RFC3339, got %q: %v", name, generatedAt, err)
	}

	if _, ok := doc["data"]; !ok {
		t.Fatalf("%s: expected data field", name)
	}

	_, has := doc["pagination"]
	if has != hasPagination {
		t.Fatalf("%s: pagination presence=%v, expected %v", name, has, hasPagination)
	}

	if hasPagination {
		pagination := contractObject(t, doc, "pagination")
		for _, field := range []string{"limit", "offset", "total", "returned"} {
			contractNumber(t, pagination, field)
		}
	}

	if _, ok := doc["error"]; ok {
		t.Fatalf("%s: success examples/responses must not include error field", name)
	}
}

func contractValidateExampleData(t *testing.T, name string, kind string, data any) {
	t.Helper()

	switch kind {
	case "config":
		obj := contractAsObject(t, name, data)
		cluster := contractObject(t, obj, "cluster")
		graph := contractObject(t, obj, "graph")
		contractString(t, cluster, "name")
		for _, field := range []string{"secrets", "workloads", "edges"} {
			contractNumber(t, graph, field)
		}

	case "secrets-list":
		items := contractAsArray(t, name, data)
		for i, item := range items {
			contractValidateSecretSummary(t, fmt.Sprintf("%s[%d]", name, i), contractAsObject(t, name, item))
		}

	case "workloads-list":
		items := contractAsArray(t, name, data)
		for i, item := range items {
			contractValidateWorkloadSummary(t, fmt.Sprintf("%s[%d]", name, i), contractAsObject(t, name, item))
		}

	case "dependency-map":
		obj := contractAsObject(t, name, data)
		nodes := contractArray(t, obj, "nodes")
		edges := contractArray(t, obj, "edges")

		for i, node := range nodes {
			contractValidateDependencyMapNode(t, fmt.Sprintf("%s.nodes[%d]", name, i), contractAsObject(t, name, node))
		}

		for i, edge := range edges {
			contractValidateDependencyEdge(t, fmt.Sprintf("%s.edges[%d]", name, i), contractAsObject(t, name, edge))
		}

	case "secret-impact":
		obj := contractAsObject(t, name, data)

		if _, ok := obj["workloads"]; ok {
			t.Fatalf("%s: secret impact must not expose deprecated workloads field", name)
		}

		contractValidateSecretSummary(t, name+".secret", contractObject(t, obj, "secret"))
		summary := contractObject(t, obj, "summary")
		for _, field := range []string{"affectedWorkloadCount", "affectedTeamCount", "affectedNamespaceCount", "unresolvedReferenceCount"} {
			contractNumber(t, summary, field)
		}
		contractObject(t, summary, "discoveryModes")

		for i, team := range contractArray(t, obj, "affectedTeams") {
			teamObj := contractAsObject(t, name, team)
			contractString(t, teamObj, "ownerTeam")
			contractNumber(t, teamObj, "workloadCount")
			_ = i
		}

		for i, workload := range contractArray(t, obj, "affectedWorkloads") {
			contractValidateWorkloadSummary(t, fmt.Sprintf("%s.affectedWorkloads[%d]", name, i), contractAsObject(t, name, workload))
		}

		for i, edge := range contractArray(t, obj, "edges") {
			contractValidateDependencyEdge(t, fmt.Sprintf("%s.edges[%d]", name, i), contractAsObject(t, name, edge))
		}

	case "workload-dependencies":
		obj := contractAsObject(t, name, data)

		if _, ok := obj["secrets"]; ok {
			t.Fatalf("%s: workload dependencies must not expose deprecated secrets field", name)
		}
		if _, ok := obj["edges"]; ok {
			t.Fatalf("%s: workload dependencies must not expose deprecated edges field", name)
		}

		contractValidateWorkloadSummary(t, name+".workload", contractObject(t, obj, "workload"))

		for i, dep := range contractArray(t, obj, "dependencies") {
			depObj := contractAsObject(t, name, dep)
			contractValidateSecretSummary(t, fmt.Sprintf("%s.dependencies[%d].secret", name, i), contractObject(t, depObj, "secret"))
			contractString(t, depObj, "discoveryMode")
			contractBool(t, depObj, "resolved")
			contractBool(t, depObj, "optional")
			sources := contractArray(t, depObj, "sources")
			for sourceIndex, source := range sources {
				contractValidateDependencySource(t, fmt.Sprintf("%s.dependencies[%d].sources[%d]", name, i, sourceIndex), contractAsObject(t, name, source))
			}
		}

	default:
		t.Fatalf("%s: unknown contract kind %q", name, kind)
	}
}

func contractValidateDependencyMapNode(t *testing.T, name string, node map[string]any) {
	t.Helper()

	contractString(t, node, "id")
	nodeType := contractString(t, node, "type")
	if nodeType != "workload" && nodeType != "secret" {
		t.Fatalf("%s: node.type must be workload or secret, got %q", name, nodeType)
	}
	contractString(t, node, "label")
	ref := contractObject(t, node, "ref")

	switch nodeType {
	case "workload":
		contractValidateWorkloadRef(t, name+".ref", ref)
	case "secret":
		contractValidateSecretRef(t, name+".ref", ref)
	}
}

func contractValidateDependencyEdge(t *testing.T, name string, edge map[string]any) {
	t.Helper()

	if _, ok := edge["from"]; ok {
		t.Fatalf("%s: edge must not expose deprecated from field", name)
	}
	if _, ok := edge["to"]; ok {
		t.Fatalf("%s: edge must not expose deprecated to field", name)
	}

	contractString(t, edge, "id")
	contractString(t, edge, "cluster")
	contractValidateWorkloadRef(t, name+".workload", contractObject(t, edge, "workload"))
	contractValidateSecretRef(t, name+".secret", contractObject(t, edge, "secret"))
	contractString(t, edge, "discoveryMode")
	contractBool(t, edge, "optional")
	contractBool(t, edge, "resolved")

	for i, source := range contractArray(t, edge, "sources") {
		contractValidateDependencySource(t, fmt.Sprintf("%s.sources[%d]", name, i), contractAsObject(t, name, source))
	}
}

func contractValidateDependencySource(t *testing.T, name string, source map[string]any) {
	t.Helper()

	if got := contractString(t, source, "type"); strings.TrimSpace(got) == "" {
		t.Fatalf("%s: expected non-empty source.type", name)
	}
	if got := contractString(t, source, "path"); strings.TrimSpace(got) == "" {
		t.Fatalf("%s: expected non-empty source.path", name)
	}
}

func contractValidateSecretSummary(t *testing.T, name string, summary map[string]any) {
	t.Helper()

	contractValidateSecretRef(t, name+".ref", contractObject(t, summary, "ref"))
	contractBool(t, summary, "exists")

	for _, field := range []string{
		"observedChangeCount",
		"affectedWorkloadCount",
		"affectedTeamCount",
		"affectedNamespaceCount",
		"unresolvedReferenceCount",
		"impactScore",
	} {
		contractNumber(t, summary, field)
	}
}

func contractValidateWorkloadSummary(t *testing.T, name string, summary map[string]any) {
	t.Helper()

	contractValidateWorkloadRef(t, name+".ref", contractObject(t, summary, "ref"))
	contractString(t, summary, "discoveryMode")
	contractNumber(t, summary, "dependencyCount")
	contractNumber(t, summary, "unresolvedCount")
}

func contractValidateSecretRef(t *testing.T, name string, ref map[string]any) {
	t.Helper()

	for _, field := range []string{"cluster", "namespace", "name"} {
		if got := contractString(t, ref, field); strings.TrimSpace(got) == "" {
			t.Fatalf("%s: expected non-empty %s", name, field)
		}
	}
}

func contractValidateWorkloadRef(t *testing.T, name string, ref map[string]any) {
	t.Helper()

	for _, field := range []string{"cluster", "namespace", "apiVersion", "kind", "name"} {
		if got := contractString(t, ref, field); strings.TrimSpace(got) == "" {
			t.Fatalf("%s: expected non-empty %s", name, field)
		}
	}
}

func contractLoadOpenAPI(t *testing.T) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(contractRepoPath("docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI file: %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse OpenAPI YAML: %v", err)
	}

	return doc
}

func contractChartVersion(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile(contractRepoPath("charts", "kbeacon", "Chart.yaml"))
	if err != nil {
		t.Fatalf("read Chart.yaml: %v", err)
	}

	re := regexp.MustCompile(`(?m)^version:\s*([^\s]+)\s*$`)
	match := re.FindSubmatch(raw)
	if len(match) != 2 {
		t.Fatalf("could not find chart version in Chart.yaml")
	}

	return string(match[1])
}

func contractRepoPath(parts ...string) string {
	all := append([]string{"..", ".."}, parts...)
	return filepath.Join(all...)
}

func contractRequirePaths(t *testing.T, doc map[string]any, expected []string) {
	t.Helper()

	paths := contractObject(t, doc, "paths")
	for _, path := range expected {
		pathObj := contractObject(t, paths, path)
		contractObject(t, pathObj, "get")
	}
}

func contractRequireParams(t *testing.T, doc map[string]any, path string, expected []string) {
	t.Helper()

	names := contractParameterNames(t, doc, path)
	for _, name := range expected {
		if !names[name] {
			t.Fatalf("%s: expected query/path parameter %q; got %v", path, name, sortedContractKeys(names))
		}
	}
}

func contractParameterNames(t *testing.T, doc map[string]any, path string) map[string]bool {
	t.Helper()

	paths := contractObject(t, doc, "paths")
	pathObj := contractObject(t, paths, path)
	getObj := contractObject(t, pathObj, "get")

	rawParams, ok := getObj["parameters"].([]any)
	if !ok {
		return map[string]bool{}
	}

	out := map[string]bool{}

	for _, raw := range rawParams {
		paramObj := contractAsObject(t, path, raw)

		if ref, _ := paramObj["$ref"].(string); ref != "" {
			paramObj = contractResolveRef(t, doc, ref)
		}

		name := contractString(t, paramObj, "name")
		out[name] = true
	}

	return out
}

func contractRequireUniqueOperationIDs(t *testing.T, doc map[string]any) {
	t.Helper()

	paths := contractObject(t, doc, "paths")
	seen := map[string]string{}

	for path, rawPathObj := range paths {
		pathObj := contractAsObject(t, path, rawPathObj)
		for method, rawOperationObj := range pathObj {
			if strings.HasPrefix(method, "x-") {
				continue
			}

			operationObj, ok := rawOperationObj.(map[string]any)
			if !ok {
				continue
			}

			operationID, ok := operationObj["operationId"].(string)
			if !ok || strings.TrimSpace(operationID) == "" {
				t.Fatalf("%s %s: missing operationId", strings.ToUpper(method), path)
			}

			location := fmt.Sprintf("%s %s", strings.ToUpper(method), path)
			if previous, exists := seen[operationID]; exists {
				t.Fatalf("operationId %q is duplicated by %s and %s", operationID, previous, location)
			}

			seen[operationID] = location
		}
	}
}

func contractRequireSchemaProps(t *testing.T, schemas map[string]any, schemaName string, required []string, forbidden []string) {
	t.Helper()

	schema := contractAsObject(t, schemaName, schemas[schemaName])
	props := contractObject(t, schema, "properties")

	for _, field := range required {
		if _, ok := props[field]; !ok {
			t.Fatalf("schema %s missing property %s", schemaName, field)
		}
	}

	for _, field := range forbidden {
		if _, ok := props[field]; ok {
			t.Fatalf("schema %s must not expose deprecated property %s", schemaName, field)
		}
	}
}

func contractResolveRef(t *testing.T, doc map[string]any, ref string) map[string]any {
	t.Helper()

	if !strings.HasPrefix(ref, "#/") {
		t.Fatalf("unsupported non-local OpenAPI ref %q", ref)
	}

	var current any = doc
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		obj := contractAsObject(t, ref, current)
		next, ok := obj[part]
		if !ok {
			t.Fatalf("OpenAPI ref %q missing part %q", ref, part)
		}
		current = next
	}

	return contractAsObject(t, ref, current)
}

func contractObject(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := parent[key]
	if !ok {
		t.Fatalf("missing object field %q in %#v", key, parent)
	}

	return contractAsObject(t, key, value)
}

func contractArray(t *testing.T, parent map[string]any, key string) []any {
	t.Helper()

	value, ok := parent[key]
	if !ok {
		t.Fatalf("missing array field %q in %#v", key, parent)
	}

	return contractAsArray(t, key, value)
}

func contractString(t *testing.T, parent map[string]any, key string) string {
	t.Helper()

	value, ok := parent[key]
	if !ok {
		t.Fatalf("missing string field %q in %#v", key, parent)
	}

	out, ok := value.(string)
	if !ok {
		t.Fatalf("field %q must be a string, got %#v", key, value)
	}

	return out
}

func contractBool(t *testing.T, parent map[string]any, key string) bool {
	t.Helper()

	value, ok := parent[key]
	if !ok {
		t.Fatalf("missing bool field %q in %#v", key, parent)
	}

	out, ok := value.(bool)
	if !ok {
		t.Fatalf("field %q must be a bool, got %#v", key, value)
	}

	return out
}

func contractNumber(t *testing.T, parent map[string]any, key string) float64 {
	t.Helper()

	value, ok := parent[key]
	if !ok {
		t.Fatalf("missing number field %q in %#v", key, parent)
	}

	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case uint64:
		return float64(typed)
	default:
		t.Fatalf("field %q must be a number, got %#v", key, value)
		return 0
	}
}

func contractAsObject(t *testing.T, name string, value any) map[string]any {
	t.Helper()

	out, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s must be an object, got %#v", name, value)
	}

	return out
}

func contractAsArray(t *testing.T, name string, value any) []any {
	t.Helper()

	out, ok := value.([]any)
	if !ok {
		t.Fatalf("%s must be an array, got %#v", name, value)
	}

	return out
}

func sortedContractKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
