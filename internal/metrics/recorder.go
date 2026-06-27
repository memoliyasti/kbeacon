package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	recorderResources = []string{
		"Secret",
		"Pod",
		"Deployment",
		"StatefulSet",
		"DaemonSet",
		"Job",
		"CronJob",
	}

	recorderEvents = []string{
		"add",
		"update",
		"delete",
	}

	recorderReasons = []string{
		"initial-sync",
		"add",
		"update",
		"delete",
	}
)

type RuntimeRecorder struct {
	cluster string

	watchEvents         *prometheus.CounterVec
	graphUpdateDuration *prometheus.HistogramVec
}

func NewRuntimeRecorder(cluster string) *RuntimeRecorder {
	r := &RuntimeRecorder{
		cluster: cluster,
		watchEvents: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "kbeacon_kubernetes_watch_events_total",
				Help: "Kubernetes informer events observed by KBeacon.",
				ConstLabels: prometheus.Labels{
					"cluster": cluster,
				},
			},
			[]string{"resource", "event"},
		),
		graphUpdateDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "kbeacon_graph_update_duration_seconds",
				Help: "Duration of KBeacon dependency graph rebuilds.",
				ConstLabels: prometheus.Labels{
					"cluster": cluster,
				},
				Buckets: []float64{
					0.0005,
					0.001,
					0.0025,
					0.005,
					0.01,
					0.025,
					0.05,
					0.1,
					0.25,
					0.5,
					1,
					2.5,
					5,
					10,
				},
			},
			[]string{"reason"},
		),
	}

	r.initializeSeries()
	return r
}

func (r *RuntimeRecorder) initializeSeries() {
	for _, resource := range recorderResources {
		for _, event := range recorderEvents {
			r.watchEvents.WithLabelValues(resource, event)
		}
	}

	for _, reason := range recorderReasons {
		r.graphUpdateDuration.WithLabelValues(reason)
	}
}

func (r *RuntimeRecorder) ObserveWatchEvent(resource, event string) {
	if r == nil {
		return
	}
	r.watchEvents.WithLabelValues(resource, event).Inc()
}

func (r *RuntimeRecorder) ObserveGraphUpdate(reason string, duration time.Duration) {
	if r == nil {
		return
	}
	r.graphUpdateDuration.WithLabelValues(reason).Observe(duration.Seconds())
}

func (r *RuntimeRecorder) Describe(ch chan<- *prometheus.Desc) {
	r.watchEvents.Describe(ch)
	r.graphUpdateDuration.Describe(ch)
}

func (r *RuntimeRecorder) Collect(ch chan<- prometheus.Metric) {
	r.watchEvents.Collect(ch)
	r.graphUpdateDuration.Collect(ch)
}
