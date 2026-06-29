# Dashboard queries

## Scrape label portability

KBeacon domain metrics include a `cluster` label. The Prometheus `job` label depends on whether you use ServiceMonitor, Service annotations, or static scrape config. Use Grafana variables such as `$cluster` and `$job` instead of hard-coding a single job name.


This page collects PromQL snippets used by the KBeacon dashboards and alerting examples.

KBeacon dashboards are intentionally Prometheus first. The Agent discovers the graph, Prometheus stores the current and historical metric samples, and Grafana renders the operational view.

## Agent health

    up{job=~"$job"}

Use this to confirm Prometheus can scrape the Agent.

## Graph size

    kbeacon_cluster_dependency_count{cluster=~"$cluster"}
    kbeacon_cluster_secret_count{cluster=~"$cluster"}
    kbeacon_cluster_workload_count{cluster=~"$cluster"}

These metrics show the current graph size by cluster.

## Highest impact Secrets

    topk(20, kbeacon_secret_impact_score{cluster=~"$cluster"})

Use this panel to find Secrets that should receive extra review before rotation.

## Secrets with broad fan-out

    topk(20, kbeacon_secret_affected_workload_count{cluster=~"$cluster"})

This highlights Secrets referenced by many workloads.

## Unresolved Secret references

    kbeacon_unresolved_secret_references{cluster=~"$cluster"} > 0

This catches workloads that reference missing Secrets or Secrets that are not observable in low-privilege mode.

## Workload dependency count

    topk(20, kbeacon_workload_dependency_count{cluster=~"$cluster"})

This helps identify workloads with many Secret dependencies.

## Detailed dependency edges

    kbeacon_dependency_edges{cluster=~"$cluster"}

`kbeacon_dependency_edges` is the detailed edge metric. It includes workload and Secret names as labels. Keep it enabled for small and medium clusters when you want graph panels and troubleshooting detail.

For large clusters or shared Prometheus environments, disable detailed edge metrics:

    helm upgrade --install kbeacon ./charts/kbeacon --namespace kbeacon-system --set cluster.name=prod-eu-1 --set metrics.edge.enabled=false

Aggregate metrics and the Agent API remain available when detailed edge metrics are disabled.

## Graph rebuild latency

    histogram_quantile(0.95, sum by (cluster, le) (rate(kbeacon_graph_update_duration_seconds_bucket{cluster=~"$cluster"}[5m])))

Use this to understand how long graph rebuilds take after Kubernetes watch events.

## Watch event rate

    sum by (cluster, resource, event) (rate(kbeacon_kubernetes_watch_events_total{cluster=~"$cluster"}[5m]))

Use this to see informer event pressure by resource type.

## Cache sync status

    min by (cluster, resource) (kbeacon_cache_sync_status{cluster=~"$cluster"})

This should stay at `1` for enabled informers. Disabled resources are not emitted in this metric.

## Demo validation

The blast-radius demo includes a live metrics check:

    make demo-metrics-live

The dashboard JSON files can be validated locally with:

    make dashboards-lint
