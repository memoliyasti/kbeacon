package graph

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type Cache struct {
	mu             sync.RWMutex
	cluster        string
	secrets        map[string]SecretSummary
	workloads      map[string]WorkloadSummary
	edges          map[string]DependencyEdge
	secretVersions map[string]string
}

func NewCache(cluster string) *Cache {
	return &Cache{
		cluster:        cluster,
		secrets:        map[string]SecretSummary{},
		workloads:      map[string]WorkloadSummary{},
		edges:          map[string]DependencyEdge{},
		secretVersions: map[string]string{},
	}
}

func EdgeID(workload WorkloadRef, secret SecretRef) string {
	return strings.Join([]string{
		workload.Cluster,
		workload.Namespace,
		strings.ToLower(workload.Kind),
		workload.Name,
		secret.Namespace,
		secret.Name,
	}, "|")
}

func SecretID(ref SecretRef) string {
	return ref.Namespace + "/" + ref.Name
}

func WorkloadID(ref WorkloadRef) string {
	return ref.Namespace + "/" + strings.ToLower(ref.Kind) + "/" + ref.Name
}

func (c *Cache) ApplySnapshot(secretInputs []SecretInput, workloadInputs []WorkloadInput, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newSecrets := make(map[string]SecretSummary, len(secretInputs))
	newVersions := make(map[string]string, len(secretInputs))
	newWorkloads := make(map[string]WorkloadSummary, len(workloadInputs))
	newEdges := make(map[string]DependencyEdge)

	for _, input := range secretInputs {
		ref := input.Ref
		ref.Cluster = c.cluster
		ref.Key = ""

		key := SecretID(ref)
		prev := c.secrets[key]
		prevVersion := c.secretVersions[key]

		changeCount := prev.ObservedChangeCount
		changeTime := prev.LastObservedChangeTime

		if changeTime.IsZero() {
			changeTime = input.CreationTimestamp
			if changeTime.IsZero() {
				changeTime = now
			}
		}

		if prevVersion != "" && input.ResourceVersion != "" && prevVersion != input.ResourceVersion {
			changeCount++
			changeTime = now
		}

		newSecrets[key] = SecretSummary{
			Ref:                    ref,
			Exists:                 true,
			Type:                   input.Type,
			OwnerTeam:              input.OwnerTeam,
			Criticality:            normalizeCriticality(input.Criticality),
			LastObservedChangeTime: changeTime,
			ObservedChangeCount:    changeCount,
		}
		newVersions[key] = input.ResourceVersion
	}

	for _, input := range workloadInputs {
		ref := input.Ref
		ref.Cluster = c.cluster

		wkey := WorkloadID(ref)
		newWorkloads[wkey] = WorkloadSummary{
			Ref:           ref,
			OwnerTeam:     input.OwnerTeam,
			Service:       input.Service,
			Environment:   input.Environment,
			Criticality:   normalizeCriticality(input.Criticality),
			DiscoveryMode: input.DiscoveryMode,
		}

		for _, edge := range input.Edges {
			edge.Cluster = c.cluster
			edge.Workload.Cluster = c.cluster
			edge.Secret.Cluster = c.cluster
			edge.Secret.Key = ""
			edge.ID = EdgeID(edge.Workload, edge.Secret)

			if edge.OwnerTeam == "" {
				edge.OwnerTeam = input.OwnerTeam
			}
			if edge.Criticality == "" {
				edge.Criticality = normalizeCriticality(input.Criticality)
			}
			if edge.DiscoveryMode == "" {
				edge.DiscoveryMode = input.DiscoveryMode
			}

			if _, ok := newSecrets[SecretID(edge.Secret)]; ok {
				edge.Resolved = true
			} else {
				edge.Resolved = false
			}

			if existing, ok := c.edges[edge.ID]; ok && !existing.FirstObservedAt.IsZero() {
				edge.FirstObservedAt = existing.FirstObservedAt
			} else {
				edge.FirstObservedAt = now
			}
			edge.LastObservedAt = now

			if existing, ok := newEdges[edge.ID]; ok {
				existing.Sources = append(existing.Sources, edge.Sources...)
				existing.Optional = existing.Optional && edge.Optional
				existing.Resolved = existing.Resolved || edge.Resolved
				if existing.DiscoveryMode != edge.DiscoveryMode {
					existing.DiscoveryMode = DiscoveryModeHybrid
				}
				newEdges[edge.ID] = existing
				continue
			}

			newEdges[edge.ID] = edge
		}
	}

	secretAffectedWorkloads := map[string]map[string]struct{}{}
	secretAffectedTeams := map[string]map[string]struct{}{}
	secretAffectedNamespaces := map[string]map[string]struct{}{}
	secretDerivedOwnerTeams := map[string]map[string]struct{}{}
	secretDerivedCriticality := map[string]string{}
	secretUnresolvedRefs := map[string]int{}
	workloadDeps := map[string]map[string]struct{}{}
	workloadUnresolved := map[string]int{}

	for _, edge := range newEdges {
		skey := SecretID(edge.Secret)
		wkey := WorkloadID(edge.Workload)

		if _, ok := newSecrets[skey]; !ok {
			newSecrets[skey] = SecretSummary{
				Ref:         edge.Secret,
				Exists:      false,
				Criticality: "unknown",
			}
		}

		if _, ok := secretAffectedWorkloads[skey]; !ok {
			secretAffectedWorkloads[skey] = map[string]struct{}{}
		}
		secretAffectedWorkloads[skey][wkey] = struct{}{}

		if _, ok := secretAffectedTeams[skey]; !ok {
			secretAffectedTeams[skey] = map[string]struct{}{}
		}
		if edge.OwnerTeam != "" {
			secretAffectedTeams[skey][edge.OwnerTeam] = struct{}{}

			if _, ok := secretDerivedOwnerTeams[skey]; !ok {
				secretDerivedOwnerTeams[skey] = map[string]struct{}{}
			}
			secretDerivedOwnerTeams[skey][edge.OwnerTeam] = struct{}{}
		}

		secretDerivedCriticality[skey] = maxCriticality(secretDerivedCriticality[skey], edge.Criticality)

		if _, ok := secretAffectedNamespaces[skey]; !ok {
			secretAffectedNamespaces[skey] = map[string]struct{}{}
		}
		secretAffectedNamespaces[skey][edge.Workload.Namespace] = struct{}{}

		if _, ok := workloadDeps[wkey]; !ok {
			workloadDeps[wkey] = map[string]struct{}{}
		}
		workloadDeps[wkey][skey] = struct{}{}

		if !edge.Resolved {
			secretUnresolvedRefs[skey]++
			workloadUnresolved[wkey]++
		}
	}

	for skey, secret := range newSecrets {
		if secret.OwnerTeam == "" {
			secret.OwnerTeam = singleTeam(secretDerivedOwnerTeams[skey])
		}

		derivedCriticality := secretDerivedCriticality[skey]
		if derivedCriticality != "" {
			secret.Criticality = maxCriticality(secret.Criticality, derivedCriticality)
		}

		secret.AffectedWorkloadCount = len(secretAffectedWorkloads[skey])
		secret.AffectedTeamCount = len(secretAffectedTeams[skey])
		secret.AffectedNamespaceCount = len(secretAffectedNamespaces[skey])
		secret.UnresolvedReferenceCount = secretUnresolvedRefs[skey]
		secret.ImpactScore = impactScore(secret)
		newSecrets[skey] = secret
	}

	for wkey, workload := range newWorkloads {
		workload.DependencyCount = len(workloadDeps[wkey])
		workload.UnresolvedCount = workloadUnresolved[wkey]
		newWorkloads[wkey] = workload
	}

	c.secrets = newSecrets
	c.workloads = newWorkloads
	c.edges = newEdges
	c.secretVersions = newVersions
}

func (c *Cache) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	secrets := make([]SecretSummary, 0, len(c.secrets))
	for _, secret := range c.secrets {
		secrets = append(secrets, secret)
	}
	sort.Slice(secrets, func(i, j int) bool {
		return SecretID(secrets[i].Ref) < SecretID(secrets[j].Ref)
	})

	workloads := make([]WorkloadSummary, 0, len(c.workloads))
	for _, workload := range c.workloads {
		workloads = append(workloads, workload)
	}
	sort.Slice(workloads, func(i, j int) bool {
		return WorkloadID(workloads[i].Ref) < WorkloadID(workloads[j].Ref)
	})

	edges := make([]DependencyEdge, 0, len(c.edges))
	for _, edge := range c.edges {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		return edges[i].ID < edges[j].ID
	})

	return Snapshot{
		Cluster:     c.cluster,
		GeneratedAt: time.Now().UTC(),
		Secrets:     secrets,
		Workloads:   workloads,
		Edges:       edges,
	}
}

func (c *Cache) ListSecrets() []SecretSummary {
	return c.Snapshot().Secrets
}

func (c *Cache) ListWorkloads() []WorkloadSummary {
	return c.Snapshot().Workloads
}

func (c *Cache) DependencyMap() DependencyMap {
	snapshot := c.Snapshot()

	nodes := make([]DependencyMapNode, 0, len(snapshot.Secrets)+len(snapshot.Workloads))

	for _, workload := range snapshot.Workloads {
		nodes = append(nodes, DependencyMapNode{
			ID:          "workload:" + WorkloadID(workload.Ref),
			Type:        "workload",
			Label:       workload.Ref.Kind + "/" + workload.Ref.Name,
			Ref:         workload.Ref,
			OwnerTeam:   workload.OwnerTeam,
			Criticality: workload.Criticality,
		})
	}

	for _, secret := range snapshot.Secrets {
		nodes = append(nodes, DependencyMapNode{
			ID:          "secret:" + SecretID(secret.Ref),
			Type:        "secret",
			Label:       "Secret/" + secret.Ref.Name,
			Ref:         secret.Ref,
			OwnerTeam:   secret.OwnerTeam,
			Criticality: secret.Criticality,
			ImpactScore: secret.ImpactScore,
		})
	}

	return DependencyMap{
		Nodes: nodes,
		Edges: snapshot.Edges,
	}
}

func (c *Cache) GetSecretImpact(namespace, name string) (SecretImpact, bool) {
	snapshot := c.Snapshot()
	targetKey := namespace + "/" + name

	var secret SecretSummary
	found := false
	workloadsByKey := map[string]WorkloadSummary{}
	teams := map[string]map[string]struct{}{}
	modeCounts := map[string]int{}
	edges := []DependencyEdge{}
	affected := []WorkloadSummary{}

	for _, item := range snapshot.Secrets {
		if SecretID(item.Ref) == targetKey {
			secret = item
			found = true
			break
		}
	}

	for _, workload := range snapshot.Workloads {
		workloadsByKey[WorkloadID(workload.Ref)] = workload
	}

	seenWorkloads := map[string]struct{}{}
	for _, edge := range snapshot.Edges {
		if SecretID(edge.Secret) != targetKey {
			continue
		}

		edges = append(edges, edge)
		modeCounts[string(edge.DiscoveryMode)]++

		wkey := WorkloadID(edge.Workload)
		workload, ok := workloadsByKey[wkey]
		if !ok {
			continue
		}

		if _, seen := seenWorkloads[wkey]; !seen {
			affected = append(affected, workload)
			seenWorkloads[wkey] = struct{}{}
		}

		if workload.OwnerTeam != "" {
			if _, ok := teams[workload.OwnerTeam]; !ok {
				teams[workload.OwnerTeam] = map[string]struct{}{}
			}
			teams[workload.OwnerTeam][wkey] = struct{}{}
		}
	}

	if !found && len(edges) == 0 {
		return SecretImpact{}, false
	}

	affectedTeams := make([]AffectedTeam, 0, len(teams))
	for team, workloads := range teams {
		affectedTeams = append(affectedTeams, AffectedTeam{
			OwnerTeam:     team,
			WorkloadCount: len(workloads),
		})
	}
	sort.Slice(affectedTeams, func(i, j int) bool {
		return affectedTeams[i].OwnerTeam < affectedTeams[j].OwnerTeam
	})

	return SecretImpact{
		Secret: secret,
		Summary: SecretImpactSummary{
			AffectedWorkloadCount:    secret.AffectedWorkloadCount,
			AffectedTeamCount:        secret.AffectedTeamCount,
			AffectedNamespaceCount:   secret.AffectedNamespaceCount,
			UnresolvedReferenceCount: secret.UnresolvedReferenceCount,
			DiscoveryModes:           modeCounts,
		},
		AffectedTeams:     affectedTeams,
		AffectedWorkloads: affected,
		Edges:             edges,
	}, true
}

func (c *Cache) GetWorkloadDependencies(namespace, kind, name string) (WorkloadDependencies, bool) {
	snapshot := c.Snapshot()

	var workload WorkloadSummary
	found := false

	for _, item := range snapshot.Workloads {
		if item.Ref.Namespace != namespace || item.Ref.Name != name {
			continue
		}
		if kind != "" && !strings.EqualFold(item.Ref.Kind, kind) {
			continue
		}
		workload = item
		found = true
		break
	}

	if !found {
		return WorkloadDependencies{}, false
	}

	secretsByKey := map[string]SecretSummary{}
	for _, secret := range snapshot.Secrets {
		secretsByKey[SecretID(secret.Ref)] = secret
	}

	deps := []WorkloadDependency{}
	for _, edge := range snapshot.Edges {
		if WorkloadID(edge.Workload) != WorkloadID(workload.Ref) {
			continue
		}

		secret := secretsByKey[SecretID(edge.Secret)]
		deps = append(deps, WorkloadDependency{
			Secret:        secret,
			DiscoveryMode: edge.DiscoveryMode,
			Resolved:      edge.Resolved,
			Optional:      edge.Optional,
			Sources:       edge.Sources,
		})
	}

	return WorkloadDependencies{
		Workload:     workload,
		Dependencies: deps,
	}, true
}

func maxCriticality(a, b string) string {
	if criticalityRank(b) > criticalityRank(a) {
		return normalizeCriticality(b)
	}
	return normalizeCriticality(a)
}

func criticalityRank(value string) int {
	switch normalizeCriticality(value) {
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	case "critical":
		return 4
	default:
		return 0
	}
}

func singleTeam(teams map[string]struct{}) string {
	if len(teams) != 1 {
		return ""
	}

	for team := range teams {
		return team
	}

	return ""
}

func normalizeCriticality(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high", "critical":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func impactScore(secret SecretSummary) float64 {
	score := float64(secret.AffectedWorkloadCount * 2)
	score += float64(secret.AffectedTeamCount * 2)
	score += float64(secret.AffectedNamespaceCount * 2)
	score += float64(secret.UnresolvedReferenceCount * 2)

	switch secret.Criticality {
	case "low":
		score += 2
	case "medium":
		score += 8
	case "high":
		score += 18
	case "critical":
		score += 30
	}

	return math.Min(100, score)
}
