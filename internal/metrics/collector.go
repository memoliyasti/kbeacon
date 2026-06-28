package metrics

import (
	"runtime"
	"strconv"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"github.com/prometheus/client_golang/prometheus"
)

type GraphCollectorOptions struct {
	EmitEdges bool
}

func DefaultGraphCollectorOptions() GraphCollectorOptions {
	return GraphCollectorOptions{
		EmitEdges: true,
	}
}

type GraphCollector struct {
	graph     *graph.Cache
	cluster   string
	version   string
	commit    string
	emitEdges bool

	buildInfo                   *prometheus.Desc
	clusterDependencyCount      *prometheus.Desc
	clusterSecretCount          *prometheus.Desc
	clusterWorkloadCount        *prometheus.Desc
	dependencyEdges             *prometheus.Desc
	workloadDependencyCount     *prometheus.Desc
	secretAffectedWorkloadCount *prometheus.Desc
	secretImpactScore           *prometheus.Desc
	secretLastChangedTimestamp  *prometheus.Desc
	secretChangesTotal          *prometheus.Desc
	secretInfo                  *prometheus.Desc
	unresolvedSecretReferences  *prometheus.Desc
}

func NewGraphCollector(cache *graph.Cache, cluster, version, commit string) *GraphCollector {
	return NewGraphCollectorWithOptions(cache, cluster, version, commit, DefaultGraphCollectorOptions())
}

func NewGraphCollectorWithOptions(cache *graph.Cache, cluster, version, commit string, options GraphCollectorOptions) *GraphCollector {
	return &GraphCollector{
		graph:     cache,
		cluster:   cluster,
		version:   version,
		commit:    commit,
		emitEdges: options.EmitEdges,

		buildInfo: prometheus.NewDesc(
			"kbeacon_build_info",
			"KBeacon build information.",
			[]string{"version", "commit", "go_version"},
			nil,
		),
		clusterDependencyCount: prometheus.NewDesc(
			"kbeacon_cluster_dependency_count",
			"Total dependency edges in the cluster.",
			[]string{"cluster"},
			nil,
		),
		clusterSecretCount: prometheus.NewDesc(
			"kbeacon_cluster_secret_count",
			"Observed Kubernetes Secret count.",
			[]string{"cluster"},
			nil,
		),
		clusterWorkloadCount: prometheus.NewDesc(
			"kbeacon_cluster_workload_count",
			"Observed workload count.",
			[]string{"cluster"},
			nil,
		),
		dependencyEdges: prometheus.NewDesc(
			"kbeacon_dependency_edges",
			"Current Secret dependency edge. Value is always 1 for active edges.",
			[]string{"cluster", "workload_namespace", "workload_kind", "workload_name", "secret_namespace", "secret_name", "discovery_mode", "owner_team", "criticality", "resolved", "optional"},
			nil,
		),
		workloadDependencyCount: prometheus.NewDesc(
			"kbeacon_workload_dependency_count",
			"Unique Secret dependency count by workload.",
			[]string{"cluster", "namespace", "workload_kind", "workload_name", "owner_team", "criticality"},
			nil,
		),
		secretAffectedWorkloadCount: prometheus.NewDesc(
			"kbeacon_secret_affected_workload_count",
			"Unique workload count affected by Secret.",
			[]string{"cluster", "namespace", "secret_name", "owner_team", "criticality", "exists"},
			nil,
		),
		secretImpactScore: prometheus.NewDesc(
			"kbeacon_secret_impact_score",
			"Secret impact score from 0 to 100.",
			[]string{"cluster", "namespace", "secret_name", "owner_team", "criticality", "exists"},
			nil,
		),
		secretLastChangedTimestamp: prometheus.NewDesc(
			"kbeacon_secret_last_changed_timestamp_seconds",
			"Last observed Secret change timestamp as Unix seconds.",
			[]string{"cluster", "namespace", "secret_name"},
			nil,
		),
		secretChangesTotal: prometheus.NewDesc(
			"kbeacon_secret_changes_total",
			"Observed Secret metadata update count.",
			[]string{"cluster", "namespace", "secret_name"},
			nil,
		),
		secretInfo: prometheus.NewDesc(
			"kbeacon_secret_info",
			"Secret metadata info. Value is always 1.",
			[]string{"cluster", "namespace", "secret_name", "type", "owner_team", "criticality", "exists"},
			nil,
		),
		unresolvedSecretReferences: prometheus.NewDesc(
			"kbeacon_unresolved_secret_references",
			"Unresolved Secret references by Secret.",
			[]string{"cluster", "namespace", "secret_name"},
			nil,
		),
	}
}

func (c *GraphCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.buildInfo
	ch <- c.clusterDependencyCount
	ch <- c.clusterSecretCount
	ch <- c.clusterWorkloadCount
	if c.emitEdges {
		ch <- c.dependencyEdges
	}
	ch <- c.workloadDependencyCount
	ch <- c.secretAffectedWorkloadCount
	ch <- c.secretImpactScore
	ch <- c.secretLastChangedTimestamp
	ch <- c.secretChangesTotal
	ch <- c.secretInfo
	ch <- c.unresolvedSecretReferences
}

func (c *GraphCollector) Collect(ch chan<- prometheus.Metric) {
	snapshot := c.graph.Snapshot()

	ch <- prometheus.MustNewConstMetric(
		c.buildInfo,
		prometheus.GaugeValue,
		1,
		c.version,
		c.commit,
		runtime.Version(),
	)

	ch <- prometheus.MustNewConstMetric(
		c.clusterDependencyCount,
		prometheus.GaugeValue,
		float64(len(snapshot.Edges)),
		c.cluster,
	)

	observedSecretCount := 0
	for _, secret := range snapshot.Secrets {
		if secret.Exists {
			observedSecretCount++
		}
	}

	ch <- prometheus.MustNewConstMetric(
		c.clusterSecretCount,
		prometheus.GaugeValue,
		float64(observedSecretCount),
		c.cluster,
	)

	ch <- prometheus.MustNewConstMetric(
		c.clusterWorkloadCount,
		prometheus.GaugeValue,
		float64(len(snapshot.Workloads)),
		c.cluster,
	)

	if c.emitEdges {
		for _, edge := range snapshot.Edges {
			ch <- prometheus.MustNewConstMetric(
				c.dependencyEdges,
				prometheus.GaugeValue,
				1,
				edge.Cluster,
				edge.Workload.Namespace,
				edge.Workload.Kind,
				edge.Workload.Name,
				edge.Secret.Namespace,
				edge.Secret.Name,
				string(edge.DiscoveryMode),
				edge.OwnerTeam,
				edge.Criticality,
				strconv.FormatBool(edge.Resolved),
				strconv.FormatBool(edge.Optional),
			)
		}
	}

	for _, workload := range snapshot.Workloads {
		ch <- prometheus.MustNewConstMetric(
			c.workloadDependencyCount,
			prometheus.GaugeValue,
			float64(workload.DependencyCount),
			workload.Ref.Cluster,
			workload.Ref.Namespace,
			workload.Ref.Kind,
			workload.Ref.Name,
			workload.OwnerTeam,
			workload.Criticality,
		)
	}

	for _, secret := range snapshot.Secrets {
		exists := strconv.FormatBool(secret.Exists)

		ch <- prometheus.MustNewConstMetric(
			c.secretAffectedWorkloadCount,
			prometheus.GaugeValue,
			float64(secret.AffectedWorkloadCount),
			secret.Ref.Cluster,
			secret.Ref.Namespace,
			secret.Ref.Name,
			secret.OwnerTeam,
			secret.Criticality,
			exists,
		)

		ch <- prometheus.MustNewConstMetric(
			c.secretImpactScore,
			prometheus.GaugeValue,
			secret.ImpactScore,
			secret.Ref.Cluster,
			secret.Ref.Namespace,
			secret.Ref.Name,
			secret.OwnerTeam,
			secret.Criticality,
			exists,
		)

		ch <- prometheus.MustNewConstMetric(
			c.secretInfo,
			prometheus.GaugeValue,
			1,
			secret.Ref.Cluster,
			secret.Ref.Namespace,
			secret.Ref.Name,
			secret.Type,
			secret.OwnerTeam,
			secret.Criticality,
			exists,
		)

		if !secret.LastObservedChangeTime.IsZero() {
			ch <- prometheus.MustNewConstMetric(
				c.secretLastChangedTimestamp,
				prometheus.GaugeValue,
				float64(secret.LastObservedChangeTime.Unix()),
				secret.Ref.Cluster,
				secret.Ref.Namespace,
				secret.Ref.Name,
			)
		}

		ch <- prometheus.MustNewConstMetric(
			c.secretChangesTotal,
			prometheus.CounterValue,
			float64(secret.ObservedChangeCount),
			secret.Ref.Cluster,
			secret.Ref.Namespace,
			secret.Ref.Name,
		)

		if secret.UnresolvedReferenceCount > 0 {
			ch <- prometheus.MustNewConstMetric(
				c.unresolvedSecretReferences,
				prometheus.GaugeValue,
				float64(secret.UnresolvedReferenceCount),
				secret.Ref.Cluster,
				secret.Ref.Namespace,
				secret.Ref.Name,
			)
		}
	}
}
