package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type snapshotDiffDocument struct {
	APIVersion string                          `json:"apiVersion"`
	Kind       string                          `json:"kind"`
	Old        string                          `json:"old"`
	New        string                          `json:"new"`
	Summary    map[string]snapshotDiffSummary  `json:"summary"`
	Resources  map[string]snapshotResourceDiff `json:"resources"`
}

type snapshotDiffSummary struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
}

type snapshotResourceDiff struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

func runSnapshotDiff(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("snapshot diff", flag.ContinueOnError)
	fs.SetOutput(stderr)

	format := fs.String("format", "text", "Output format: text or json")
	include := fs.String("include", "secrets,workloads,edges", "Comma-separated resources to diff: secrets,workloads,edges")
	failOnChange := fs.Bool("fail-on-change", false, "Exit with status 1 when any difference is detected")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "usage: kbeaconctl snapshot diff [--format text|json] [--include LIST] [--fail-on-change] <old-snapshot.json> <new-snapshot.json>")
		return 2
	}

	resources := parseDiffResources(*include)
	if len(resources) == 0 {
		fmt.Fprintln(stderr, "snapshot diff include list is empty")
		return 2
	}

	doc, err := buildSnapshotDiff(fs.Arg(0), fs.Arg(1), resources)
	if err != nil {
		fmt.Fprintf(stderr, "snapshot diff failed: %v\n", err)
		return 1
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(doc); err != nil {
			fmt.Fprintf(stderr, "write snapshot diff json: %v\n", err)
			return 1
		}
	case "text", "":
		writeSnapshotDiffText(stdout, doc, resources)
	default:
		fmt.Fprintf(stderr, "unsupported snapshot diff format %q\n", *format)
		return 2
	}

	if *failOnChange && snapshotDiffHasChanges(doc) {
		return 1
	}

	return 0
}

func buildSnapshotDiff(oldPath, newPath string, resources []string) (snapshotDiffDocument, error) {
	oldDoc, err := readSnapshotDocument(oldPath)
	if err != nil {
		return snapshotDiffDocument{}, fmt.Errorf("read old snapshot: %w", err)
	}

	newDoc, err := readSnapshotDocument(newPath)
	if err != nil {
		return snapshotDiffDocument{}, fmt.Errorf("read new snapshot: %w", err)
	}

	result := snapshotDiffDocument{
		APIVersion: "kbeacon.io/v1",
		Kind:       "KBeaconSnapshotDiff",
		Old:        oldPath,
		New:        newPath,
		Summary:    map[string]snapshotDiffSummary{},
		Resources:  map[string]snapshotResourceDiff{},
	}

	for _, resource := range resources {
		var oldIndex map[string]string
		var newIndex map[string]string

		switch resource {
		case "secrets":
			oldIndex = indexSnapshotItems(extractSnapshotArray(oldDoc, "secrets"), snapshotSecretID)
			newIndex = indexSnapshotItems(extractSnapshotArray(newDoc, "secrets"), snapshotSecretID)
		case "workloads":
			oldIndex = indexSnapshotItems(extractSnapshotArray(oldDoc, "workloads"), snapshotWorkloadID)
			newIndex = indexSnapshotItems(extractSnapshotArray(newDoc, "workloads"), snapshotWorkloadID)
		case "edges":
			oldIndex = indexSnapshotItems(extractSnapshotArray(oldDoc, "edges"), snapshotEdgeID)
			newIndex = indexSnapshotItems(extractSnapshotArray(newDoc, "edges"), snapshotEdgeID)
		default:
			return snapshotDiffDocument{}, fmt.Errorf("unsupported snapshot resource %q", resource)
		}

		diff := diffSnapshotIndex(oldIndex, newIndex)
		result.Resources[resource] = diff
		result.Summary[resource] = snapshotDiffSummary{
			Added:   len(diff.Added),
			Removed: len(diff.Removed),
			Changed: len(diff.Changed),
		}
	}

	return result, nil
}

func readSnapshotDocument(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}

	if strings.TrimSpace(fmt.Sprint(doc["kind"])) != "KBeaconSnapshot" {
		return nil, errors.New("input is not a KBeaconSnapshot document")
	}

	return doc, nil
}

func parseDiffResources(value string) []string {
	seen := map[string]struct{}{}
	out := []string{}

	for _, raw := range strings.Split(value, ",") {
		item := strings.TrimSpace(strings.ToLower(raw))
		if item == "" {
			continue
		}

		switch item {
		case "secret":
			item = "secrets"
		case "workload":
			item = "workloads"
		case "edge":
			item = "edges"
		}

		if _, ok := seen[item]; ok {
			continue
		}

		seen[item] = struct{}{}
		out = append(out, item)
	}

	return out
}

func extractSnapshotArray(doc map[string]any, resource string) []any {
	paths := snapshotResourcePaths(resource)

	for _, path := range paths {
		value, ok := lookupSnapshotPath(doc, path...)
		if !ok {
			continue
		}

		if arr, ok := value.([]any); ok {
			return arr
		}
	}

	return nil
}

func snapshotResourcePaths(resource string) [][]string {
	switch resource {
	case "secrets":
		return [][]string{
			{"resources", "secrets", "data"},
			{"data", "resources", "secrets", "data"},
			{"data", "secrets"},
			{"resources", "secrets"},
			{"secrets"},
		}
	case "workloads":
		return [][]string{
			{"resources", "workloads", "data"},
			{"data", "resources", "workloads", "data"},
			{"data", "workloads"},
			{"resources", "workloads"},
			{"workloads"},
		}
	case "edges":
		return [][]string{
			{"resources", "dependency-map", "data", "edges"},
			{"resources", "dependencyMap", "data", "edges"},
			{"data", "resources", "dependency-map", "data", "edges"},
			{"data", "resources", "dependencyMap", "data", "edges"},
			{"data", "dependency-map", "edges"},
			{"data", "dependencyMap", "edges"},
			{"dependency-map", "edges"},
			{"dependencyMap", "edges"},
			{"data", "edges"},
			{"edges"},
		}
	default:
		return nil
	}
}

func lookupSnapshotPath(root any, path ...string) (any, bool) {
	current := root

	for _, segment := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}

		next, ok := m[segment]
		if !ok {
			return nil, false
		}

		current = next
	}

	return current, true
}

func indexSnapshotItems(items []any, idFunc func(any) string) map[string]string {
	out := map[string]string{}

	for _, item := range items {
		id := idFunc(item)
		if id == "" {
			continue
		}

		out[id] = canonicalSnapshotJSON(item)
	}

	return out
}

func diffSnapshotIndex(oldIndex, newIndex map[string]string) snapshotResourceDiff {
	diff := snapshotResourceDiff{
		Added:   []string{},
		Removed: []string{},
		Changed: []string{},
	}

	for id, newValue := range newIndex {
		oldValue, ok := oldIndex[id]
		if !ok {
			diff.Added = append(diff.Added, id)
			continue
		}

		if oldValue != newValue {
			diff.Changed = append(diff.Changed, id)
		}
	}

	for id := range oldIndex {
		if _, ok := newIndex[id]; !ok {
			diff.Removed = append(diff.Removed, id)
		}
	}

	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)
	sort.Strings(diff.Changed)

	return diff
}

func snapshotSecretID(value any) string {
	m, ok := value.(map[string]any)
	if !ok {
		return ""
	}

	if ref, ok := m["ref"].(map[string]any); ok {
		namespace := snapshotString(ref["namespace"])
		name := snapshotString(ref["name"])
		if namespace != "" && name != "" {
			return namespace + "/" + name
		}
	}

	namespace := snapshotString(m["namespace"])
	name := snapshotString(m["name"])
	if namespace != "" && name != "" {
		return namespace + "/" + name
	}

	return ""
}

func snapshotWorkloadID(value any) string {
	m, ok := value.(map[string]any)
	if !ok {
		return ""
	}

	if ref, ok := m["ref"].(map[string]any); ok {
		namespace := snapshotString(ref["namespace"])
		kind := strings.ToLower(snapshotString(ref["kind"]))
		name := snapshotString(ref["name"])
		if namespace != "" && kind != "" && name != "" {
			return namespace + "/" + kind + "/" + name
		}
	}

	namespace := snapshotString(m["namespace"])
	kind := strings.ToLower(snapshotString(m["kind"]))
	name := snapshotString(m["name"])
	if namespace != "" && kind != "" && name != "" {
		return namespace + "/" + kind + "/" + name
	}

	return ""
}

func snapshotEdgeID(value any) string {
	m, ok := value.(map[string]any)
	if !ok {
		return ""
	}

	if id := snapshotString(m["id"]); id != "" {
		return id
	}

	workloadID := ""
	if workload, ok := m["workload"]; ok {
		workloadID = snapshotWorkloadID(workload)
	}

	secretID := ""
	if secret, ok := m["secret"]; ok {
		secretID = snapshotSecretID(secret)
	}

	if workloadID != "" && secretID != "" {
		return workloadID + "->" + secretID
	}

	return ""
}

func snapshotString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func canonicalSnapshotJSON(value any) string {
	normalized := normalizeSnapshotValue(value)

	raw, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Sprint(value)
	}

	return string(raw)
}

func normalizeSnapshotValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range typed {
			if key == "generatedAt" {
				continue
			}
			out[key] = normalizeSnapshotValue(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeSnapshotValue(item))
		}
		return out
	default:
		return value
	}
}

func writeSnapshotDiffText(w io.Writer, doc snapshotDiffDocument, resources []string) {
	fmt.Fprintln(w, "KBeacon Snapshot Diff")
	fmt.Fprintf(w, "Old: %s\n", doc.Old)
	fmt.Fprintf(w, "New: %s\n", doc.New)
	fmt.Fprintln(w)

	if !snapshotDiffHasChanges(doc) {
		fmt.Fprintln(w, "No changes detected.")
		return
	}

	for _, resource := range resources {
		diff, ok := doc.Resources[resource]
		if !ok {
			continue
		}

		summary := doc.Summary[resource]
		fmt.Fprintf(w, "%s: +%d -%d ~%d\n", snapshotResourceLabel(resource), summary.Added, summary.Removed, summary.Changed)

		for _, id := range diff.Added {
			fmt.Fprintf(w, "  + %s\n", id)
		}
		for _, id := range diff.Removed {
			fmt.Fprintf(w, "  - %s\n", id)
		}
		for _, id := range diff.Changed {
			fmt.Fprintf(w, "  ~ %s\n", id)
		}

		fmt.Fprintln(w)
	}
}

func snapshotResourceLabel(resource string) string {
	switch resource {
	case "secrets":
		return "Secrets"
	case "workloads":
		return "Workloads"
	case "edges":
		return "Edges"
	default:
		return resource
	}
}

func snapshotDiffHasChanges(doc snapshotDiffDocument) bool {
	for _, summary := range doc.Summary {
		if summary.Added > 0 || summary.Removed > 0 || summary.Changed > 0 {
			return true
		}
	}
	return false
}
