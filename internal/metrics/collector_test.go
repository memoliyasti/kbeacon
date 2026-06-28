package metrics

import (
	"testing"
	"time"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestGraphCollectorEmitsEdgeSeriesByDefault(t *testing.T) {
	families := gatherGraphCollector(t, true)

	if !metricFamilyExists(families, "kbeacon_dependency_edges") {
		t.Fatal("expected kbeacon_dependency_edges metric family when edge metrics are enabled")
	}

	if !metricFamilyExists(families, "kbeacon_cluster_dependency_count") {
		t.Fatal("expected kbeacon_cluster_dependency_count metric family")
	}
}

func TestGraphCollectorCanDisableEdgeSeries(t *testing.T) {
	families := gatherGraphCollector(t, false)

	if metricFamilyExists(families, "kbeacon_dependency_edges") {
		t.Fatal("did not expect kbeacon_dependency_edges metric family when edge metrics are disabled")
	}

	if !metricFamilyExists(families, "kbeacon_cluster_dependency_count") {
		t.Fatal("expected aggregate dependency count to remain available")
	}
}

func gatherGraphCollector(t *testing.T, emitEdges bool) []*dto.MetricFamily {
	t.Helper()

	cache := graph.NewCache("minikube")
	now := time.Unix(1000, 0)

	workload := graph.WorkloadRef{
		Cluster:    "minikube",
		Namespace:  "payments",
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "api",
	}
	secret := graph.SecretRef{
		Cluster:   "minikube",
		Namespace: "payments",
		Name:      "db-password",
	}

	cache.ApplySnapshot(
		[]graph.SecretInput{
			{
				Ref:               secret,
				Type:              "Opaque",
				ResourceVersion:   "1",
				CreationTimestamp: now,
			},
		},
		[]graph.WorkloadInput{
			{
				Ref:           workload,
				OwnerTeam:     "payments",
				Criticality:   "high",
				DiscoveryMode: graph.DiscoveryModeHybrid,
				Edges: []graph.DependencyEdge{
					{
						Workload:      workload,
						Secret:        secret,
						DiscoveryMode: graph.DiscoveryModeInfer,
						Sources: []graph.DependencySource{
							{
								Type: "env.secretKeyRef",
								Path: "env[DB_PASSWORD].valueFrom.secretKeyRef",
							},
						},
					},
				},
			},
		},
		now,
	)

	registry := prometheus.NewRegistry()
	registry.MustRegister(NewGraphCollectorWithOptions(
		cache,
		"minikube",
		"test",
		"test",
		GraphCollectorOptions{EmitEdges: emitEdges},
	))

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	return families
}

func metricFamilyExists(families []*dto.MetricFamily, name string) bool {
	for _, family := range families {
		if family.GetName() == name {
			return true
		}
	}
	return false
}
