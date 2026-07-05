package discovery

import (
	"fmt"
	"sort"
	"strings"
)

func crdSortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func crdMapFieldPath(parent, key string) string {
	return fmt.Sprintf("%s[%q]", parent, key)
}

func crdJoinPath(parts ...string) string {
	var out strings.Builder

	for _, part := range parts {
		if part == "" {
			continue
		}

		if strings.HasPrefix(part, "[") {
			out.WriteString(part)
			continue
		}

		if out.Len() > 0 {
			out.WriteByte('.')
		}
		out.WriteString(part)
	}

	return out.String()
}
