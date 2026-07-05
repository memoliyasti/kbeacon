package metrics

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/memoliyasti/kbeacon/internal/graph"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestGraphCollectorMetricLabelContracts(t *testing.T) {
	cache := metricContractGraph(t)

	families := gatherMetricFamilies(
		t,
		NewGraphCollectorWithOptions(
			cache,
			"compat-cluster",
			"test-version",
			"test-commit",
			GraphCollectorOptions{EmitEdges: true},
		),
	)

	expected := map[string][]string{
		"kbeacon_cluster_dependency_count":              {"cluster"},
		"kbeacon_cluster_secret_count":                  {"cluster"},
		"kbeacon_cluster_workload_count":                {"cluster"},
		"kbeacon_dependency_edges":                      {"cluster", "workload_namespace", "workload_kind", "workload_name", "secret_namespace", "secret_name", "discovery_mode", "owner_team", "criticality", "resolved", "optional"},
		"kbeacon_workload_dependency_count":             {"cluster", "namespace", "workload_kind", "workload_name", "owner_team", "criticality"},
		"kbeacon_secret_affected_workload_count":        {"cluster", "namespace", "secret_name", "owner_team", "criticality", "exists"},
		"kbeacon_secret_impact_score":                   {"cluster", "namespace", "secret_name", "owner_team", "criticality", "exists"},
		"kbeacon_secret_last_changed_timestamp_seconds": {"cluster", "namespace", "secret_name"},
		"kbeacon_secret_changes_total":                  {"cluster", "namespace", "secret_name"},
		"kbeacon_secret_info":                           {"cluster", "namespace", "secret_name", "type", "owner_team", "criticality", "exists"},
		"kbeacon_unresolved_secret_references":          {"cluster", "namespace", "secret_name"},
	}

	for family, labels := range expected {
		assertMetricFamilyLabelSet(t, families, family, labels)
	}
}

func TestGraphCollectorMetricLabelsAvoidSourcePathCardinality(t *testing.T) {
	cache := metricContractGraph(t)

	families := gatherMetricFamilies(
		t,
		NewGraphCollectorWithOptions(
			cache,
			"compat-cluster",
			"test-version",
			"test-commit",
			GraphCollectorOptions{EmitEdges: true},
		),
	)

	forbiddenLabels := map[string]struct{}{
		"uid":         {},
		"pod_name":    {},
		"secret_key":  {},
		"source_path": {},
		"path":        {},
		"container":   {},
		"env_var":     {},
		"volume":      {},
	}

	for familyName, family := range families {
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if _, forbidden := forbiddenLabels[label.GetName()]; forbidden {
					t.Fatalf("metric family %s exposes forbidden high-cardinality/source label %q", familyName, label.GetName())
				}
			}
		}
	}
}

func metricContractGraph(t *testing.T) *graph.Cache {
	t.Helper()

	cache := graph.NewCache("compat-cluster")
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)

	workload := graph.WorkloadRef{
		Cluster:    "compat-cluster",
		Namespace:  "payments",
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "api",
		UID:        "deployment-api-uid",
	}

	resolvedSecret := graph.SecretRef{
		Cluster:   "compat-cluster",
		Namespace: "payments",
		Name:      "db",
	}

	missingSecret := graph.SecretRef{
		Cluster:   "compat-cluster",
		Namespace: "payments",
		Name:      "missing-db",
	}

	workloads := []graph.WorkloadInput{
		{
			Ref:           workload,
			OwnerTeam:     "platform",
			Service:       "payments-api",
			Environment:   "prod",
			Criticality:   "high",
			DiscoveryMode: graph.DiscoveryModeHybrid,
			Edges: []graph.DependencyEdge{
				{
					Workload:      workload,
					Secret:        resolvedSecret,
					DiscoveryMode: graph.DiscoveryModeInfer,
					Optional:      false,
					Sources: []graph.DependencySource{
						{
							Type:      "env.secretKeyRef",
							Path:      "spec.template.spec.containers[api].env[DB_PASSWORD].valueFrom.secretKeyRef",
							Container: "api",
							EnvVar:    "DB_PASSWORD",
						},
					},
				},
				{
					Workload:      workload,
					Secret:        missingSecret,
					DiscoveryMode: graph.DiscoveryModeExplicit,
					Optional:      true,
					Sources: []graph.DependencySource{
						{
							Type:       "annotation",
							Path:       "metadata.annotations[kbeacon.io/watch-secrets]",
							Annotation: "kbeacon.io/watch-secrets",
						},
					},
				},
			},
		},
	}

	cache.ApplySnapshot(
		[]graph.SecretInput{
			{
				Ref:               resolvedSecret,
				Type:              "Opaque",
				OwnerTeam:         "platform",
				Criticality:       "high",
				ResourceVersion:   "1",
				CreationTimestamp: now,
			},
		},
		workloads,
		now,
	)

	cache.ApplySnapshot(
		[]graph.SecretInput{
			{
				Ref:               resolvedSecret,
				Type:              "Opaque",
				OwnerTeam:         "platform",
				Criticality:       "high",
				ResourceVersion:   "2",
				CreationTimestamp: now,
			},
		},
		workloads,
		now.Add(time.Minute),
	)

	return cache
}

func gatherMetricFamilies(t *testing.T, collectors ...prometheus.Collector) map[string]*dto.MetricFamily {
	t.Helper()

	registry := prometheus.NewRegistry()
	for _, collector := range collectors {
		registry.MustRegister(collector)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	out := make(map[string]*dto.MetricFamily, len(families))
	for _, family := range families {
		out[family.GetName()] = family
	}

	return out
}

func assertMetricFamilyLabelSet(t *testing.T, families map[string]*dto.MetricFamily, familyName string, expected []string) {
	t.Helper()

	family, ok := families[familyName]
	if !ok {
		t.Fatalf("expected metric family %s to be emitted", familyName)
	}

	if len(family.GetMetric()) == 0 {
		t.Fatalf("expected metric family %s to include at least one metric", familyName)
	}

	expectedSorted := append([]string(nil), expected...)
	sort.Strings(expectedSorted)

	for i, metric := range family.GetMetric() {
		got := metricLabelNames(metric)
		if !reflect.DeepEqual(got, expectedSorted) {
			t.Fatalf("unexpected labels for metric family %s metric %d: got %#v want %#v", familyName, i, got, expectedSorted)
		}
	}
}

func metricLabelNames(metric *dto.Metric) []string {
	labels := metric.GetLabel()
	names := make([]string, 0, len(labels))

	for _, label := range labels {
		names = append(names, label.GetName())
	}

	sort.Strings(names)
	return names
}
