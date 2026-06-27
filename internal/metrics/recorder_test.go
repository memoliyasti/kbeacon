package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRuntimeRecorderCanBeRegisteredAndObserved(t *testing.T) {
	registry := prometheus.NewRegistry()
	recorder := NewRuntimeRecorder("minikube")

	if err := registry.Register(recorder); err != nil {
		t.Fatalf("register recorder: %v", err)
	}

	recorder.ObserveWatchEvent("Secret", "add")
	recorder.ObserveWatchEvent("Deployment", "update")
	recorder.ObserveGraphUpdate("initial-sync", 10*time.Millisecond)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	foundWatchEvents := false
	foundGraphDuration := false

	for _, family := range families {
		switch family.GetName() {
		case "kbeacon_kubernetes_watch_events_total":
			foundWatchEvents = true
			if len(family.Metric) == 0 {
				t.Fatal("expected watch event metrics to contain initialized label series")
			}
		case "kbeacon_graph_update_duration_seconds":
			foundGraphDuration = true
			if len(family.Metric) == 0 {
				t.Fatal("expected graph duration histogram to contain initialized label series")
			}
		}
	}

	if !foundWatchEvents {
		t.Fatal("expected kbeacon_kubernetes_watch_events_total metric family")
	}

	if !foundGraphDuration {
		t.Fatal("expected kbeacon_graph_update_duration_seconds metric family")
	}
}
