package discovery

import (
	"fmt"
	"strings"

	"github.com/kbeacon/kbeacon/internal/graph"
)

const (
	AnnotationEnabled          = "kbeacon.io/enabled"
	AnnotationDiscoveryMode    = "kbeacon.io/discovery-mode"
	AnnotationWatchSecrets     = "kbeacon.io/watch-secrets"
	AnnotationWatchSecretsJSON = "kbeacon.io/watch-secrets-json"
	AnnotationIgnoreSecrets    = "kbeacon.io/ignore-secrets"
	AnnotationOwnerTeam        = "kbeacon.io/owner-team"
	AnnotationCriticality      = "kbeacon.io/criticality"
	AnnotationService          = "kbeacon.io/service"
	AnnotationEnvironment      = "kbeacon.io/environment"
)

func ParseWatchSecrets(cluster, workloadNamespace, value string) ([]graph.SecretRef, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	refs := make([]graph.SecretRef, 0, len(parts))
	for _, part := range parts {
		ref, err := parseSecretRef(cluster, workloadNamespace, strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func parseSecretRef(cluster, defaultNamespace, token string) (graph.SecretRef, error) {
	if token == "" {
		return graph.SecretRef{}, fmt.Errorf("empty secret reference")
	}
	ref := graph.SecretRef{Cluster: cluster, Namespace: defaultNamespace}
	namePart := token
	if slash := strings.Index(token, "/"); slash >= 0 {
		ref.Namespace = token[:slash]
		namePart = token[slash+1:]
	}
	if hash := strings.Index(namePart, "#"); hash >= 0 {
		ref.Name = namePart[:hash]
		ref.Key = namePart[hash+1:]
	} else {
		ref.Name = namePart
	}
	if ref.Namespace == "" || ref.Name == "" {
		return graph.SecretRef{}, fmt.Errorf("invalid secret reference %q", token)
	}
	return ref, nil
}
