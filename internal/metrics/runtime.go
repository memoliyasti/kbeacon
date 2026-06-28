package metrics

import (
	"github.com/memoliyasti/kbeacon/internal/graph"
	"github.com/prometheus/client_golang/prometheus"
)

type RuntimeCollector struct {
	graph   *graph.Cache
	cluster string
	version string
	commit  string
	status  func() map[string]bool
	counts  func() map[string]int

	agentInfo       *prometheus.Desc
	cacheSyncStatus *prometheus.Desc
	cacheObjects    *prometheus.Desc
}

func NewRuntimeCollector(
	cache *graph.Cache,
	cluster string,
	version string,
	commit string,
	status func() map[string]bool,
	counts func() map[string]int,
) *RuntimeCollector {
	return &RuntimeCollector{
		graph:   cache,
		cluster: cluster,
		version: version,
		commit:  commit,
		status:  status,
		counts:  counts,

		agentInfo: prometheus.NewDesc(
			"kbeacon_agent_info",
			"KBeacon Agent runtime metadata. Value is always 1.",
			[]string{"cluster", "version", "commit"},
			nil,
		),
		cacheSyncStatus: prometheus.NewDesc(
			"kbeacon_cache_sync_status",
			"Kubernetes informer cache sync status. 1 means synced, 0 means not synced.",
			[]string{"cluster", "resource"},
			nil,
		),
		cacheObjects: prometheus.NewDesc(
			"kbeacon_cache_objects",
			"Current object count in KBeacon graph cache.",
			[]string{"cluster", "resource"},
			nil,
		),
	}
}

func (c *RuntimeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.agentInfo
	ch <- c.cacheSyncStatus
	ch <- c.cacheObjects
}

func (c *RuntimeCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		c.agentInfo,
		prometheus.GaugeValue,
		1,
		c.cluster,
		c.version,
		c.commit,
	)

	if c.status != nil {
		for resource, synced := range c.status() {
			value := 0.0
			if synced {
				value = 1
			}

			ch <- prometheus.MustNewConstMetric(
				c.cacheSyncStatus,
				prometheus.GaugeValue,
				value,
				c.cluster,
				resource,
			)
		}
	}

	if c.counts != nil {
		for resource, count := range c.counts() {
			ch <- prometheus.MustNewConstMetric(
				c.cacheObjects,
				prometheus.GaugeValue,
				float64(count),
				c.cluster,
				resource,
			)
		}
		return
	}

	snapshot := c.graph.Snapshot()
	ch <- prometheus.MustNewConstMetric(c.cacheObjects, prometheus.GaugeValue, float64(len(snapshot.Secrets)), c.cluster, "Secret")
	ch <- prometheus.MustNewConstMetric(c.cacheObjects, prometheus.GaugeValue, float64(len(snapshot.Workloads)), c.cluster, "Workload")
	ch <- prometheus.MustNewConstMetric(c.cacheObjects, prometheus.GaugeValue, float64(len(snapshot.Edges)), c.cluster, "DependencyEdge")
}
