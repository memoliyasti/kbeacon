package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
)

type snapshotDiffDocument struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Old        string                 `json:"oldSnapshot"`
	New        string                 `json:"newSnapshot"`
	Summary    snapshotDiffSummary    `json:"summary"`
	Resources  []snapshotDiffResource `json:"resources"`
}

type snapshotDiffSummary struct {
	HasChanges bool `json:"hasChanges"`
	Added      int  `json:"added"`
	Removed    int  `json:"removed"`
	Changed    int  `json:"changed"`
}

type snapshotDiffResource struct {
	Name    string             `json:"name"`
	Added   []snapshotDiffItem `json:"added"`
	Removed []snapshotDiffItem `json:"removed"`
	Changed []snapshotDiffItem `json:"changed"`
}

type snapshotDiffItem struct {
	ID  string `json:"id"`
	Old any    `json:"old,omitempty"`
	New any    `json:"new,omitempty"`
}

func runSnapshotDiff(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("snapshot diff", flag.ContinueOnError)
	fs.SetOutput(stderr)

	format := fs.String("format", "text", "Output format: text, json, or markdown")
	include := fs.String("include", "secrets,workloads,edges", "Comma-separated resources to compare: secrets, workloads, edges")
	failOnChange := fs.Bool("fail-on-change", false, "Exit with status 2 when the snapshots differ")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, usage("snapshot diff [--format text|json|markdown] [--include LIST] [--fail-on-change] <old-snapshot.json> <new-snapshot.json>"))
		return 2
	}

	resources, err := parseSnapshotDiffInclude(*include)
	if err != nil {
		fmt.Fprintf(stderr, "snapshot diff include list is invalid: %v\n", err)
		return 2
	}

	diff, err := buildSnapshotDiff(fs.Arg(0), fs.Arg(1), resources)
	if err != nil {
		fmt.Fprintf(stderr, "snapshot diff failed: %v\n", err)
		return 1
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		if err := writeSnapshotDiffJSON(stdout, diff); err != nil {
			fmt.Fprintf(stderr, "write snapshot diff json: %v\n", err)
			return 1
		}
	case "markdown", "md":
		if err := writeSnapshotDiffMarkdown(stdout, diff); err != nil {
			fmt.Fprintf(stderr, "write snapshot diff markdown: %v\n", err)
			return 1
		}
	case "text", "":
		if err := writeSnapshotDiffText(stdout, diff); err != nil {
			fmt.Fprintf(stderr, "write snapshot diff text: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "unsupported snapshot diff format %q\n", *format)
		return 2
	}

	if *failOnChange && diff.Summary.HasChanges {
		return 2
	}

	return 0
}

func parseSnapshotDiffInclude(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}

	for _, part := range parts {
		resource := normalizeSnapshotDiffResource(part)
		if resource == "" {
			continue
		}

		if resource == "all" {
			for _, item := range []string{"secrets", "workloads", "edges"} {
				if _, ok := seen[item]; !ok {
					seen[item] = struct{}{}
					out = append(out, item)
				}
			}
			continue
		}

		if _, ok := seen[resource]; ok {
			continue
		}

		seen[resource] = struct{}{}
		out = append(out, resource)
	}

	if len(out) == 0 {
		return nil, errors.New("empty include list")
	}

	return out, nil
}

func normalizeSnapshotDiffResource(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "all":
		return "all"
	case "secret", "secrets":
		return "secrets"
	case "workload", "workloads":
		return "workloads"
	case "edge", "edges", "dependency", "dependencies", "dependency-map", "dependency_map":
		return "edges"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func buildSnapshotDiff(oldPath, newPath string, resources []string) (snapshotDiffDocument, error) {
	oldSnapshot, err := readSnapshotDocument(oldPath)
	if err != nil {
		return snapshotDiffDocument{}, fmt.Errorf("read old snapshot: %w", err)
	}

	newSnapshot, err := readSnapshotDocument(newPath)
	if err != nil {
		return snapshotDiffDocument{}, fmt.Errorf("read new snapshot: %w", err)
	}

	diff := snapshotDiffDocument{
		APIVersion: "kbeacon.io/v1",
		Kind:       "KBeaconSnapshotDiff",
		Old:        oldPath,
		New:        newPath,
		Resources:  make([]snapshotDiffResource, 0, len(resources)),
	}

	for _, resource := range resources {
		resourceDiff := diffSnapshotResource(resource, oldSnapshot, newSnapshot)
		diff.Summary.Added += len(resourceDiff.Added)
		diff.Summary.Removed += len(resourceDiff.Removed)
		diff.Summary.Changed += len(resourceDiff.Changed)
		diff.Resources = append(diff.Resources, resourceDiff)
	}

	diff.Summary.HasChanges = diff.Summary.Added > 0 || diff.Summary.Removed > 0 || diff.Summary.Changed > 0

	return diff, nil
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

func diffSnapshotResource(resource string, oldSnapshot, newSnapshot map[string]any) snapshotDiffResource {
	oldItems := indexSnapshotItems(resource, extractSnapshotResourceValues(oldSnapshot, resource))
	newItems := indexSnapshotItems(resource, extractSnapshotResourceValues(newSnapshot, resource))

	out := snapshotDiffResource{Name: resource}

	ids := map[string]struct{}{}
	for id := range oldItems {
		ids[id] = struct{}{}
	}
	for id := range newItems {
		ids[id] = struct{}{}
	}

	sortedIDs := make([]string, 0, len(ids))
	for id := range ids {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)

	for _, id := range sortedIDs {
		oldItem, oldOK := oldItems[id]
		newItem, newOK := newItems[id]

		switch {
		case !oldOK && newOK:
			out.Added = append(out.Added, snapshotDiffItem{ID: id, New: newItem})
		case oldOK && !newOK:
			out.Removed = append(out.Removed, snapshotDiffItem{ID: id, Old: oldItem})
		case oldOK && newOK && !reflect.DeepEqual(oldItem, newItem):
			out.Changed = append(out.Changed, snapshotDiffItem{ID: id, Old: oldItem, New: newItem})
		}
	}

	return out
}

func extractSnapshotResourceValues(snapshot map[string]any, resource string) []any {
	switch resource {
	case "secrets":
		return firstSnapshotArray(snapshot, [][]string{
			{"resources", "secrets"},
			{"resources", "secrets", "data"},
			{"data", "secrets"},
			{"secrets"},
		})
	case "workloads":
		return firstSnapshotArray(snapshot, [][]string{
			{"resources", "workloads"},
			{"resources", "workloads", "data"},
			{"data", "workloads"},
			{"workloads"},
		})
	case "edges":
		return firstSnapshotArray(snapshot, [][]string{
			{"resources", "dependency-map", "data", "edges"},
			{"resources", "dependency-map", "edges"},
			{"resources", "dependencyMap", "data", "edges"},
			{"resources", "edges"},
			{"resources", "edges", "data"},
			{"data", "edges"},
			{"edges"},
		})
	default:
		return firstSnapshotArray(snapshot, [][]string{
			{"resources", resource},
			{"resources", resource, "data"},
			{"data", resource},
			{resource},
		})
	}
}

func firstSnapshotArray(snapshot map[string]any, paths [][]string) []any {
	for _, path := range paths {
		value := getSnapshotPath(snapshot, path...)
		if values, ok := snapshotValuesFromAny(value); ok {
			return values
		}
	}

	return nil
}

func getSnapshotPath(value any, parts ...string) any {
	current := value

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
	}

	return current
}

func snapshotValuesFromAny(value any) ([]any, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case []any:
		return typed, true
	case map[string]any:
		if data, ok := typed["data"]; ok {
			if values, ok := snapshotValuesFromAny(data); ok {
				return values, true
			}
		}
		if items, ok := typed["items"]; ok {
			if values, ok := snapshotValuesFromAny(items); ok {
				return values, true
			}
		}
		if edges, ok := typed["edges"]; ok {
			if values, ok := snapshotValuesFromAny(edges); ok {
				return values, true
			}
		}
		return []any{typed}, true
	default:
		return []any{typed}, true
	}
}

func indexSnapshotItems(resource string, values []any) map[string]any {
	indexed := map[string]any{}

	for i, value := range values {
		id := snapshotItemID(resource, value)
		if id == "" {
			id = fmt.Sprintf("%s[%d]", resource, i)
		}

		originalID := id
		for collision := 2; ; collision++ {
			if _, exists := indexed[id]; !exists {
				break
			}
			id = fmt.Sprintf("%s#%d", originalID, collision)
		}

		indexed[id] = value
	}

	return indexed
}

func snapshotItemID(resource string, value any) string {
	m, ok := value.(map[string]any)
	if !ok {
		return ""
	}

	if id := snapshotStringAt(m, "id"); id != "" {
		return id
	}

	switch resource {
	case "secrets":
		if ref := snapshotMapAt(m, "ref"); ref != nil {
			return joinNonEmpty("/", snapshotStringAt(ref, "namespace"), snapshotStringAt(ref, "name"))
		}
		return joinNonEmpty("/", snapshotStringAt(m, "namespace"), snapshotStringAt(m, "name"))

	case "workloads":
		if ref := snapshotMapAt(m, "ref"); ref != nil {
			return joinNonEmpty(
				"/",
				snapshotStringAt(ref, "namespace"),
				strings.ToLower(snapshotStringAt(ref, "kind")),
				snapshotStringAt(ref, "name"),
			)
		}
		return joinNonEmpty(
			"/",
			snapshotStringAt(m, "namespace"),
			strings.ToLower(snapshotStringAt(m, "kind")),
			snapshotStringAt(m, "name"),
		)

	case "edges":
		workload := snapshotMapAt(m, "workload")
		secret := snapshotMapAt(m, "secret")

		if workload == nil || secret == nil {
			return ""
		}

		left := joinNonEmpty(
			"/",
			snapshotStringAt(workload, "namespace"),
			strings.ToLower(snapshotStringAt(workload, "kind")),
			snapshotStringAt(workload, "name"),
		)
		right := joinNonEmpty("/", snapshotStringAt(secret, "namespace"), snapshotStringAt(secret, "name"))

		if left == "" || right == "" {
			return ""
		}

		return left + "->" + right
	default:
		if ref := snapshotMapAt(m, "ref"); ref != nil {
			return snapshotItemID(resource, ref)
		}
		return snapshotStringAt(m, "name")
	}
}

func snapshotMapAt(m map[string]any, key string) map[string]any {
	value, ok := m[key].(map[string]any)
	if !ok {
		return nil
	}
	return value
}

func snapshotStringAt(m map[string]any, key string) string {
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}

	if text, ok := value.(string); ok {
		return text
	}

	return strings.TrimSpace(fmt.Sprint(value))
}

func joinNonEmpty(separator string, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, separator)
}

func writeSnapshotDiffJSON(w io.Writer, diff snapshotDiffDocument) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(diff)
}

func writeSnapshotDiffText(w io.Writer, diff snapshotDiffDocument) error {
	fmt.Fprintln(w, "KBeacon Snapshot Diff")
	fmt.Fprintf(w, "Old: %s\n", diff.Old)
	fmt.Fprintf(w, "New: %s\n", diff.New)
	fmt.Fprintln(w)

	if !diff.Summary.HasChanges {
		fmt.Fprintln(w, "No snapshot changes detected.")
		return nil
	}

	for _, resource := range diff.Resources {
		fmt.Fprintf(
			w,
			"%s: +%d -%d ~%d\n",
			snapshotDiffResourceLabel(resource.Name),
			len(resource.Added),
			len(resource.Removed),
			len(resource.Changed),
		)
		writeSnapshotDiffTextItems(w, "  +", resource.Added)
		writeSnapshotDiffTextItems(w, "  -", resource.Removed)
		writeSnapshotDiffTextItems(w, "  ~", resource.Changed)
		fmt.Fprintln(w)
	}

	return nil
}

func writeSnapshotDiffTextItems(w io.Writer, prefix string, items []snapshotDiffItem) {
	for _, item := range items {
		fmt.Fprintf(w, "%s %s\n", prefix, item.ID)
	}
}

func writeSnapshotDiffMarkdown(w io.Writer, diff snapshotDiffDocument) error {
	fmt.Fprintln(w, "## KBeacon Snapshot Diff")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "**Old snapshot:** %s  \n", snapshotMarkdownInlineCode(diff.Old))
	fmt.Fprintf(w, "**New snapshot:** %s\n", snapshotMarkdownInlineCode(diff.New))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Resource | Added | Removed | Changed |")
	fmt.Fprintln(w, "| --- | ---: | ---: | ---: |")

	for _, resource := range diff.Resources {
		fmt.Fprintf(
			w,
			"| %s | %d | %d | %d |\n",
			snapshotMarkdownTableText(snapshotDiffResourceLabel(resource.Name)),
			len(resource.Added),
			len(resource.Removed),
			len(resource.Changed),
		)
	}

	fmt.Fprintln(w)

	if !diff.Summary.HasChanges {
		fmt.Fprintln(w, "No snapshot changes detected.")
		return nil
	}

	for _, resource := range diff.Resources {
		if len(resource.Added) == 0 && len(resource.Removed) == 0 && len(resource.Changed) == 0 {
			continue
		}

		label := snapshotDiffResourceLabel(resource.Name)
		fmt.Fprintf(w, "<details>\n<summary>%s details</summary>\n\n", snapshotMarkdownTableText(label))
		writeSnapshotDiffMarkdownItems(w, "Added", resource.Added)
		writeSnapshotDiffMarkdownItems(w, "Removed", resource.Removed)
		writeSnapshotDiffMarkdownItems(w, "Changed", resource.Changed)
		fmt.Fprintln(w, "</details>")
		fmt.Fprintln(w)
	}

	return nil
}

func writeSnapshotDiffMarkdownItems(w io.Writer, label string, items []snapshotDiffItem) {
	if len(items) == 0 {
		return
	}

	fmt.Fprintf(w, "**%s**\n\n", label)

	for _, item := range items {
		fmt.Fprintf(w, "- %s\n", snapshotMarkdownInlineCode(item.ID))
	}

	fmt.Fprintln(w)
}

func snapshotDiffResourceLabel(resource string) string {
	switch resource {
	case "secrets":
		return "Secrets"
	case "workloads":
		return "Workloads"
	case "edges":
		return "Edges"
	default:
		if resource == "" {
			return "Unknown"
		}
		return strings.ToUpper(resource[:1]) + resource[1:]
	}
}

func snapshotMarkdownInlineCode(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "'") + "`"
}

func snapshotMarkdownTableText(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}
