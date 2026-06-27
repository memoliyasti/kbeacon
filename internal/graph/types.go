package graph

import "time"

type DiscoveryMode string

const (
	DiscoveryModeInfer    DiscoveryMode = "infer"
	DiscoveryModeExplicit DiscoveryMode = "explicit"
	DiscoveryModeHybrid   DiscoveryMode = "hybrid"
	DiscoveryModeDisabled DiscoveryMode = "disabled"
)

type SecretRef struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Key       string `json:"key,omitempty"`
}

type WorkloadRef struct {
	Cluster    string `json:"cluster"`
	Namespace  string `json:"namespace"`
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	UID        string `json:"uid,omitempty"`
}

type DependencySource struct {
	Type          string `json:"type"`
	Path          string `json:"path"`
	Container     string `json:"container,omitempty"`
	InitContainer string `json:"initContainer,omitempty"`
	Ephemeral     bool   `json:"ephemeralContainer,omitempty"`
	Volume        string `json:"volume,omitempty"`
	EnvVar        string `json:"envVar,omitempty"`
	Annotation    string `json:"annotation,omitempty"`
	ResourceField string `json:"resourceField,omitempty"`
}

type DependencyEdge struct {
	ID              string             `json:"id"`
	Cluster         string             `json:"cluster"`
	Workload        WorkloadRef        `json:"workload"`
	Secret          SecretRef          `json:"secret"`
	DiscoveryMode   DiscoveryMode      `json:"discoveryMode"`
	Sources         []DependencySource `json:"sources"`
	Optional        bool               `json:"optional"`
	Resolved        bool               `json:"resolved"`
	OwnerTeam       string             `json:"ownerTeam,omitempty"`
	Criticality     string             `json:"criticality,omitempty"`
	Purpose         string             `json:"purpose,omitempty"`
	FirstObservedAt time.Time          `json:"firstObservedAt,omitempty"`
	LastObservedAt  time.Time          `json:"lastObservedAt,omitempty"`
}

type SecretSummary struct {
	Ref                      SecretRef `json:"ref"`
	Exists                   bool      `json:"exists"`
	Type                     string    `json:"type,omitempty"`
	OwnerTeam                string    `json:"ownerTeam,omitempty"`
	Criticality              string    `json:"criticality,omitempty"`
	LastObservedChangeTime   time.Time `json:"lastObservedChangeTime,omitempty"`
	ObservedChangeCount      uint64    `json:"observedChangeCount"`
	AffectedWorkloadCount    int       `json:"affectedWorkloadCount"`
	AffectedTeamCount        int       `json:"affectedTeamCount"`
	AffectedNamespaceCount   int       `json:"affectedNamespaceCount"`
	UnresolvedReferenceCount int       `json:"unresolvedReferenceCount"`
	ImpactScore              float64   `json:"impactScore"`
}

type WorkloadSummary struct {
	Ref             WorkloadRef   `json:"ref"`
	OwnerTeam       string        `json:"ownerTeam,omitempty"`
	Service         string        `json:"service,omitempty"`
	Environment     string        `json:"environment,omitempty"`
	Criticality     string        `json:"criticality,omitempty"`
	DiscoveryMode   DiscoveryMode `json:"discoveryMode"`
	DependencyCount int           `json:"dependencyCount"`
	UnresolvedCount int           `json:"unresolvedCount"`
}

type SecretInput struct {
	Ref               SecretRef
	Type              string
	OwnerTeam         string
	Criticality       string
	ResourceVersion   string
	CreationTimestamp time.Time
}

type WorkloadInput struct {
	Ref           WorkloadRef
	OwnerTeam     string
	Service       string
	Environment   string
	Criticality   string
	DiscoveryMode DiscoveryMode
	Edges         []DependencyEdge
}

type Snapshot struct {
	Cluster     string            `json:"cluster"`
	GeneratedAt time.Time         `json:"generatedAt"`
	Secrets     []SecretSummary   `json:"secrets"`
	Workloads   []WorkloadSummary `json:"workloads"`
	Edges       []DependencyEdge  `json:"edges"`
}

type DependencyMapNode struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Label       string  `json:"label"`
	Ref         any     `json:"ref"`
	OwnerTeam   string  `json:"ownerTeam,omitempty"`
	Criticality string  `json:"criticality,omitempty"`
	ImpactScore float64 `json:"impactScore,omitempty"`
}

type DependencyMap struct {
	Nodes []DependencyMapNode `json:"nodes"`
	Edges []DependencyEdge    `json:"edges"`
}

type AffectedTeam struct {
	OwnerTeam     string `json:"ownerTeam"`
	WorkloadCount int    `json:"workloadCount"`
}

type SecretImpact struct {
	Secret            SecretSummary       `json:"secret"`
	Summary           SecretImpactSummary `json:"summary"`
	AffectedTeams     []AffectedTeam      `json:"affectedTeams"`
	AffectedWorkloads []WorkloadSummary   `json:"affectedWorkloads"`
	Edges             []DependencyEdge    `json:"edges"`
}

type SecretImpactSummary struct {
	AffectedWorkloadCount    int            `json:"affectedWorkloadCount"`
	AffectedTeamCount        int            `json:"affectedTeamCount"`
	AffectedNamespaceCount   int            `json:"affectedNamespaceCount"`
	UnresolvedReferenceCount int            `json:"unresolvedReferenceCount"`
	DiscoveryModes           map[string]int `json:"discoveryModes"`
}

type WorkloadDependency struct {
	Secret        SecretSummary      `json:"secret"`
	DiscoveryMode DiscoveryMode      `json:"discoveryMode"`
	Resolved      bool               `json:"resolved"`
	Optional      bool               `json:"optional"`
	Sources       []DependencySource `json:"sources"`
}

type WorkloadDependencies struct {
	Workload     WorkloadSummary      `json:"workload"`
	Dependencies []WorkloadDependency `json:"dependencies"`
}
