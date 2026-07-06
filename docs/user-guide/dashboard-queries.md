# Dashboard queries

This page documents the PromQL patterns used by KBeacon Grafana dashboards.

The dashboards are Prometheus-first. KBeacon discovers the current dependency graph, Prometheus or Grafana Mimir stores metric samples, and Grafana renders operational views.

## Query model

KBeacon dashboard queries should be portable across Prometheus integration styles.

Recommended query conventions:

- use `$datasource` for the Grafana datasource;
- use `$cluster` for KBeacon domain identity;
- use `$job` only as a scrape-job selector;
- use namespace and owner-team variables to narrow high-cardinality panels;
- avoid assuming one hard-coded Prometheus job label;
- keep `honor_labels=true` where possible so KBeacon labels are preserved.

Common dashboard variables:

| Variable | Purpose | Example source |
| --- | --- | --- |
| `$job` | Scrape job selector. | `label_values(kbeacon_build_info, job)` |
| `$cluster` | Logical KBeacon cluster selector. | `label_values(kbeacon_cluster_dependency_count{job=~"$job"}, cluster)` |
| `$namespace` | Namespace selector for workload or Secret panels. | `label_values(kbeacon_secret_affected_workload_count{job=~"$job",cluster=~"$cluster"}, namespace)` |
| `$owner_team` | Owner team selector. | `label_values(kbeacon_secret_impact_score{job=~"$job",cluster=~"$cluster"}, owner_team)` |
| `$criticality` | Criticality selector. | custom or metric label values |
| `$resolved` | Dependency resolution selector. | custom values such as `true,false` |
| `$discovery_mode` | Discovery mode selector. | custom or metric label values |
| `$window` | Rate or increase window. | custom values such as `5m,15m,1h,6h,24h` |

## Agent health

Use Prometheus scrape health to verify that the Agent target is reachable.

```promql
up{job=~"$job"}
```

For multi-cluster views, group by scrape and cluster labels when available.

```promql
max by (job, instance) (up{job=~"$job"})
```

## Graph size

Current dependency edge count by cluster.

```promql
kbeacon_cluster_dependency_count{job=~"$job",cluster=~"$cluster"}
```

Current observed Secret count by cluster.

```promql
kbeacon_cluster_secret_count{job=~"$job",cluster=~"$cluster"}
```

Current observed workload count by cluster.

```promql
kbeacon_cluster_workload_count{job=~"$job",cluster=~"$cluster"}
```

## Highest impact Secrets

Use impact score to identify Secrets that need additional review before rotation.

```promql
topk(20, kbeacon_secret_impact_score{job=~"$job",cluster=~"$cluster",namespace=~"$namespace",owner_team=~"$owner_team"})
```

## Secrets with broad fan-out

Use affected workload count to find widely referenced Secrets.

```promql
topk(20, kbeacon_secret_affected_workload_count{job=~"$job",cluster=~"$cluster",namespace=~"$namespace",owner_team=~"$owner_team"})
```

## Unresolved Secret references

Unresolved references indicate missing or unobservable Secrets.

```promql
kbeacon_unresolved_secret_references{job=~"$job",cluster=~"$cluster",namespace=~"$namespace"} > 0
```

In low-privilege mode, unobservable Secret objects are represented as unresolved because the Agent cannot confirm existence.

## Workload dependency count

Use this query to identify workloads with many Secret dependencies.

```promql
topk(20, kbeacon_workload_dependency_count{job=~"$job",cluster=~"$cluster",namespace=~"$namespace",owner_team=~"$owner_team"})
```

## Detailed dependency edges

The edge metric is used for troubleshooting tables and graph panels.

```promql
kbeacon_dependency_edges{job=~"$job",cluster=~"$cluster",workload_namespace=~"$namespace",owner_team=~"$owner_team"}
```

`kbeacon_dependency_edges` is high-cardinality because it includes workload and Secret names. Use dashboard filters before expanding large clusters.

## Dependency Graph Explorer Node Graph query

Grafana Node Graph panels need `source`, `target`, and `id` fields.

Use this query in instant table mode for the standalone `KBeacon / Dependency Graph Explorer` dashboard.

```promql
label_join(
  label_join(
    label_join(
      label_join(
        label_join(
          label_join(
            kbeacon_dependency_edges{job=~"$job",cluster=~"$cluster",workload_namespace=~"$namespace",owner_team=~"$owner_team",criticality=~"$criticality",resolved=~"$resolved",discovery_mode=~"$discovery_mode"},
            "source",
            "/",
            "workload_kind",
            "workload_namespace",
            "workload_name"
          ),
          "target",
          "/",
          "secret_namespace",
          "secret_name"
        ),
        "id",
        " -> ",
        "source",
        "target"
      ),
      "mainstat",
      " / ",
      "discovery_mode",
      "resolved"
    ),
    "detail__owner_team",
    "",
    "owner_team"
  ),
  "detail__criticality",
  "",
  "criticality"
)
```

Generated fields:

| Field | Meaning |
| --- | --- |
| `source` | Workload node id built from kind, namespace, and workload name. |
| `target` | Secret node id built from Secret namespace and name. |
| `id` | Unique edge id built from source and target. |
| `mainstat` | Compact edge status from discovery mode and resolved state. |
| `detail__owner_team` | Owner team shown as edge detail. |
| `detail__criticality` | Criticality shown as edge detail. |

The Node Graph query requires `metrics.edge.enabled=true`.

## Discovery mode distribution

Use this query to understand whether dependencies are inferred, explicit, or hybrid.

```promql
sum by (cluster, discovery_mode, resolved) (
  kbeacon_dependency_edges{job=~"$job",cluster=~"$cluster",workload_namespace=~"$namespace",owner_team=~"$owner_team"}
)
```

## Secret change activity

Observed Secret metadata updates during the selected window.

```promql
increase(kbeacon_secret_changes_total{job=~"$job",cluster=~"$cluster",namespace=~"$namespace"}[$window])
```

Age of the last observed Secret change.

```promql
time() - kbeacon_secret_last_changed_timestamp_seconds{job=~"$job",cluster=~"$cluster",namespace=~"$namespace"}
```

## Graph rebuild latency

Use this query to monitor p95 graph rebuild duration.

```promql
histogram_quantile(
  0.95,
  sum by (cluster, le) (
    rate(kbeacon_graph_update_duration_seconds_bucket{job=~"$job",cluster=~"$cluster"}[$window])
  )
)
```

## Kubernetes watch event rate

Use this query to inspect informer event pressure.

```promql
sum by (cluster, resource, event) (
  rate(kbeacon_kubernetes_watch_events_total{job=~"$job",cluster=~"$cluster"}[$window])
)
```

## Cache sync status

Enabled informers should report cache sync status as `1`.

```promql
min by (cluster, resource) (
  kbeacon_cache_sync_status{job=~"$job",cluster=~"$cluster"}
)
```

Disabled resources are not emitted in `kbeacon_cache_sync_status`.

## Edge metric disabled profile

When `metrics.edge.enabled=false`, do not use panels that depend directly on `kbeacon_dependency_edges`.

Safe aggregate metrics include:

- `kbeacon_cluster_dependency_count`;
- `kbeacon_cluster_secret_count`;
- `kbeacon_cluster_workload_count`;
- `kbeacon_secret_affected_workload_count`;
- `kbeacon_secret_impact_score`;
- `kbeacon_workload_dependency_count`;
- `kbeacon_unresolved_secret_references`.

The read-only Agent API remains available for edge-level inspection when edge metrics are disabled.

## Historical Secret timeline queries

KBeacon itself exposes current graph state. Historical views come from Prometheus or Grafana Mimir samples and recording rules.

Recent Secret update count over one hour:

~~~promql
kbeacon:secret_changes:increase_1h{cluster=~"$cluster",namespace=~"$namespace"}
~~~

Recent Secret update count over one day:

~~~promql
topk(20, kbeacon:secret_changes:increase_24h{cluster=~"$cluster",namespace=~"$namespace"})
~~~

Age since the last observed Secret update:

~~~promql
kbeacon:secret_age_since_change_seconds{cluster=~"$cluster",namespace=~"$namespace"}
~~~

Recently changed Secrets that currently affect workloads:

~~~promql
topk(20, kbeacon:recently_changed_affected_secrets{cluster=~"$cluster",namespace=~"$namespace",owner_team=~"$owner_team"})
~~~

Dependency edge trend by cluster:

~~~promql
kbeacon:dependency_edges:sum_by_cluster{cluster=~"$cluster"}
~~~

If the example recording rules are not installed, use the raw equivalents from the Secret change activity section. Recording rules are recommended for shared dashboards because they avoid repeating long PromQL expressions across panels.

## Related documentation

- Dashboard guide: `docs/user-guide/dashboards.md`
- Metrics reference: `docs/reference/metrics.md`
- Prometheus operations: `docs/operations/prometheus.md`
- Alerting guide: `docs/user-guide/alerting.md`
- Example rules: `examples/prometheus/rules.yaml`
