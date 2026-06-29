# KBeacon Metrics Reference

KBeacon exposes Prometheus metrics from the Agent `/metrics` endpoint.

The current implementation exports dependency graph metrics and runtime metrics. KBeacon does **not** export Secret values. Secret names and namespace metadata are still sensitive and should be treated accordingly.

## Label conventions

Every KBeacon domain metric includes:

| Label | Meaning |
| --- | --- |
| `cluster` | Logical cluster name from config or `KBEACON_CLUSTER_NAME`. |

Prometheus scrape jobs usually add these labels. Exact values depend on ServiceMonitor, Service annotation discovery, or static scrape config:

| Label | Meaning |
| --- | --- |
| `job` | Prometheus scrape job, for example `kbeacon-agent`. |
| `instance` | Scrape target, for example `kbeacon.kbeacon-system.svc.cluster.local:8080`. |
| `environment` | Optional static scrape label. |
| `deployment_mode` | Optional static scrape label such as `in-cluster`. |

## Implemented domain metrics

### `kbeacon_cluster_dependency_count`

Type: gauge

Total current dependency edge count by cluster.

Labels: `cluster`

```promql
sum by (cluster) (kbeacon_cluster_dependency_count)
```

### `kbeacon_cluster_secret_count`

Type: gauge

Observed Kubernetes Secret count by cluster.

Labels: `cluster`

### `kbeacon_cluster_workload_count`

Type: gauge

Observed normalized workload count by cluster.

Labels: `cluster`

### `kbeacon_dependency_edges`

Type: gauge

One current workload-to-Secret dependency edge. Value is always `1` for active edges.

Labels:

| Label | Description |
| --- | --- |
| `cluster` | Cluster name. |
| `workload_namespace` | Workload namespace. |
| `workload_kind` | Workload kind, for example `Deployment`. |
| `workload_name` | Workload name. |
| `secret_namespace` | Secret namespace. |
| `secret_name` | Secret name. |
| `discovery_mode` | `infer`, `explicit`, or `hybrid`. |
| `owner_team` | Owner team from annotations or derived graph metadata. |
| `criticality` | `unknown`, `low`, `medium`, `high`, or `critical`. |
| `resolved` | Whether the referenced Secret exists in the observed cache. |
| `optional` | Whether all merged references for the edge are optional. |

```promql
kbeacon_dependency_edges{workload_namespace="kbeacon-demo"}
```

### `kbeacon_workload_dependency_count`

Type: gauge

Unique Secret dependency count by workload.

Labels: `cluster`, `namespace`, `workload_kind`, `workload_name`, `owner_team`, `criticality`

```promql
sum by (cluster, namespace, workload_kind, workload_name) (
  kbeacon_workload_dependency_count{cluster=~"$cluster"}
)
```

### `kbeacon_secret_affected_workload_count`

Type: gauge

Unique workloads affected by a Secret.

Labels: `cluster`, `namespace`, `secret_name`, `owner_team`, `criticality`, `exists`

```promql
topk(20, kbeacon_secret_affected_workload_count{cluster=~"$cluster"})
```

### `kbeacon_secret_impact_score`

Type: gauge

Calculated Secret impact score from `0` to `100`.

Current scoring is intentionally simple and deterministic:

- fan-out contributes points;
- affected team count contributes points;
- affected namespace count contributes points;
- unresolved references contribute points;
- criticality adds a weighted score.

Labels: `cluster`, `namespace`, `secret_name`, `owner_team`, `criticality`, `exists`

```promql
topk(20, kbeacon_secret_impact_score{cluster=~"$cluster"})
```

### `kbeacon_secret_last_changed_timestamp_seconds`

Type: gauge

Last observed Secret change timestamp as Unix seconds.

Labels: `cluster`, `namespace`, `secret_name`

```promql
time() - kbeacon_secret_last_changed_timestamp_seconds{cluster=~"$cluster"}
```

### `kbeacon_secret_changes_total`

Type: counter

Observed Secret metadata update count during the current Agent lifecycle.

Labels: `cluster`, `namespace`, `secret_name`

```promql
increase(kbeacon_secret_changes_total{cluster=~"$cluster"}[1h])
```

### `kbeacon_secret_info`

Type: gauge info

Secret metadata info. Value is always `1`.

Labels: `cluster`, `namespace`, `secret_name`, `type`, `owner_team`, `criticality`, `exists`

```promql
kbeacon_secret_info{criticality=~"high|critical"}
```

### `kbeacon_unresolved_secret_references`

Type: gauge

Unresolved Secret references by Secret.

Labels: `cluster`, `namespace`, `secret_name`

```promql
kbeacon_unresolved_secret_references{cluster=~"$cluster"} > 0
```

## Low-privilege mode and `exists` / `resolved`

When `resourcesToWatch.core.secrets=false`, KBeacon does not observe Kubernetes Secret objects. It can still discover references from workloads, but it cannot prove whether a referenced Secret exists.

In this mode:

- `kbeacon_cluster_secret_count` reports observed Secret objects, so it is normally `0`;
- referenced Secret series use `exists="false"`;
- dependency edge series use `resolved="false"`;
- `kbeacon_secret_last_changed_timestamp_seconds` is not emitted for unobserved Secrets;
- `kbeacon_secret_changes_total` remains `0` for unobserved Secrets.

Treat `exists="false"` and `resolved="false"` as "missing or unobservable" when Secret watching is disabled.

## Cardinality guard

`kbeacon_dependency_edges` is the most detailed metric family. It includes workload names and Secret names, so it can produce many time series in large clusters.

Disable edge metrics when you only need aggregate impact, high fan-out, unresolved reference, and runtime health metrics:

```yaml
metrics:
  edge:
    enabled: false
  runtime:
    enabled: true
```

When edge metrics are disabled:

- `kbeacon_dependency_edges` is not emitted;
- aggregate metrics such as `kbeacon_cluster_dependency_count`, `kbeacon_workload_dependency_count`, `kbeacon_secret_affected_workload_count`, `kbeacon_secret_impact_score`, and `kbeacon_unresolved_secret_references` remain available;
- the Agent API still returns dependency edges for internal debugging and inspection;
- Grafana panels that rely directly on `kbeacon_dependency_edges` should be disabled or treated as optional.

This setting is useful for very large clusters, shared Prometheus environments, or organizations that consider workload and Secret names sensitive metadata.

## Implemented runtime metrics

### `kbeacon_build_info`

Type: gauge info

Build metadata. Value is always `1`.

Labels: `version`, `commit`, `go_version`

### `kbeacon_agent_info`

Type: gauge info

Agent runtime metadata. Value is always `1`.

Labels: `cluster`, `version`, `commit`

### `kbeacon_cache_sync_status`

Type: gauge

Kubernetes informer cache sync status. `1` means synced, `0` means not synced.

Labels: `cluster`, `resource`

Disabled resources are not emitted in this metric.

```promql
min by (cluster, resource) (kbeacon_cache_sync_status{cluster=~"$cluster"})
```

### `kbeacon_cache_objects`

Type: gauge

Current object count in the KBeacon graph cache.

Labels: `cluster`, `resource`

`resource` is one of `Secret`, `Workload`, or `DependencyEdge`.

### `kbeacon_kubernetes_watch_events_total`

Type: counter

Kubernetes informer events observed by KBeacon.

Labels: `cluster`, `resource`, `event`

`event` is one of `add`, `update`, or `delete`.

```promql
sum by (cluster, resource, event) (
  rate(kbeacon_kubernetes_watch_events_total{cluster=~"$cluster"}[5m])
)
```

### `kbeacon_graph_update_duration_seconds`

Type: histogram

Duration of dependency graph rebuilds.

Labels: `cluster`, `reason`

`reason` is one of `initial-sync`, `add`, `update`, or `delete`.

```promql
histogram_quantile(
  0.95,
  sum by (cluster, le) (
    rate(kbeacon_graph_update_duration_seconds_bucket{cluster=~"$cluster"}[5m])
  )
)
```

## Metrics not implemented yet

The following metrics are described in older design material but are not currently exported by the Agent:

- `kbeacon_team_dependency_count`
- `kbeacon_team_affected_secret_count`
- `kbeacon_workload_info`
- `kbeacon_secret_affected_team_count`
- `kbeacon_secret_affected_namespace_count`
- `kbeacon_kubernetes_watch_errors_total`
- `kbeacon_reconcile_duration_seconds`
- `kbeacon_http_requests_total`
- `kbeacon_http_request_duration_seconds`
- `kbeacon_annotation_parse_errors_total`
- `kbeacon_discovery_mode_workload_count`

Use currently implemented metrics and the REST API for detailed source information.

## Cardinality rules

KBeacon intentionally avoids these labels by default:

- Secret key;
- environment variable name;
- full source path;
- container name;
- Pod UID;
- Kubernetes resourceVersion;
- arbitrary Kubernetes labels;
- arbitrary Kubernetes annotations;
- raw error message text.

Use the REST API for detailed dependency source paths and Secret key-level details.

## Useful PromQL

Agent health:

```promql
up{cluster=~"$cluster"}
```

Top impact Secrets:

```promql
topk(20, kbeacon_secret_impact_score{cluster=~"$cluster"})
```

Secrets affecting workloads:

```promql
kbeacon_secret_affected_workload_count{cluster=~"$cluster"} > 0
```

Dependency edges for a namespace:

```promql
kbeacon_dependency_edges{cluster=~"$cluster",workload_namespace="kbeacon-demo"}
```

Recent Secret changes:

```promql
increase(kbeacon_secret_changes_total{cluster=~"$cluster"}[1h])
```

Graph rebuild p95:

```promql
histogram_quantile(
  0.95,
  sum by (cluster, le) (
    rate(kbeacon_graph_update_duration_seconds_bucket{cluster=~"$cluster"}[5m])
  )
)
```
