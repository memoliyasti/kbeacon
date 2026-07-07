# Metrics Reference

KBeacon exposes Prometheus metrics from the Agent HTTP endpoint at `/metrics`.

The metric contract is designed for Prometheus and Grafana Mimir storage, Grafana dashboards, alerting rules, and platform automation.

KBeacon metrics do not expose Kubernetes Secret values. They can include Secret names, workload names, namespaces, ownership metadata, and dependency state, so access should be protected as operational metadata.

## Metric compatibility policy

Prometheus metric family names and label sets are compatibility-sensitive.

KBeacon tests the public graph metric label sets so high-cardinality implementation details such as Secret keys, dependency source paths, container names, environment variable names, UIDs, and Pod instance names do not become Prometheus labels accidentally.

See [Compatibility](compatibility.md) for the project-wide API, metrics, and Helm compatibility policy.

## Label conventions

Every KBeacon domain metric includes the logical cluster label.

| Label | Meaning |
| --- | --- |
| `cluster` | Logical cluster name from KBeacon configuration. |

Prometheus adds scrape labels depending on the scrape integration profile.

| Label | Meaning |
| --- | --- |
| `job` | Prometheus scrape job. |
| `instance` | Prometheus scrape target. |

Dashboard queries should use `$cluster` for KBeacon domain identity and `$job` only as a scrape selector.

## Domain metrics

### `kbeacon_cluster_dependency_count`

Type: gauge

Current number of workload-to-Secret dependency edges in the graph.

Labels: `cluster`

```promql
kbeacon_cluster_dependency_count{cluster=~"$cluster"}
```

### `kbeacon_cluster_secret_count`

Type: gauge

Current number of observed or referenced Secrets in the graph.

Labels: `cluster`

```promql
kbeacon_cluster_secret_count{cluster=~"$cluster"}
```

### `kbeacon_cluster_workload_count`

Type: gauge

Current number of normalized workloads in the graph.

Labels: `cluster`

```promql
kbeacon_cluster_workload_count{cluster=~"$cluster"}
```

### `kbeacon_dependency_edges`

Type: gauge

One active workload-to-Secret dependency edge. The value is always `1` for an active edge.

Labels:

| Label | Meaning |
| --- | --- |
| `cluster` | Logical cluster name. |
| `workload_namespace` | Workload namespace. |
| `workload_kind` | Workload kind, such as `Deployment` or `CronJob`. |
| `workload_name` | Workload name. |
| `secret_namespace` | Secret namespace. |
| `secret_name` | Secret name. |
| `discovery_mode` | `infer`, `explicit`, or `hybrid`. |
| `owner_team` | Workload owner team metadata when available. |
| `criticality` | Workload or Secret criticality classification. |
| `resolved` | Whether the referenced Secret exists in the observed cache. |
| `optional` | Whether all merged references for the edge are optional. |

```promql
kbeacon_dependency_edges{cluster=~"$cluster",workload_namespace=~"$namespace"}
```

This is the highest-cardinality KBeacon metric family because it includes workload and Secret names.

Grafana Node Graph panels require this metric. If `metrics.edge.enabled=false`, this metric is not emitted and edge-level graph panels are empty.

### `kbeacon_workload_dependency_count`

Type: gauge

Unique Secret dependency count by workload.

Labels: `cluster`, `namespace`, `workload_kind`, `workload_name`, `owner_team`, `criticality`

```promql
topk(20, kbeacon_workload_dependency_count{cluster=~"$cluster",namespace=~"$namespace"})
```

### `kbeacon_secret_affected_workload_count`

Type: gauge

Number of unique workloads affected by a Secret.

Labels: `cluster`, `namespace`, `secret_name`, `owner_team`, `criticality`, `exists`

```promql
topk(20, kbeacon_secret_affected_workload_count{cluster=~"$cluster",namespace=~"$namespace"})
```

### `kbeacon_secret_impact_score`

Type: gauge

Calculated Secret impact score from `0` to `100`.

The current scoring model is deterministic and based on fan-out, affected owner teams, affected namespaces, unresolved references, and criticality.

Labels: `cluster`, `namespace`, `secret_name`, `owner_team`, `criticality`, `exists`

```promql
topk(20, kbeacon_secret_impact_score{cluster=~"$cluster",namespace=~"$namespace",owner_team=~"$owner_team"})
```

### `kbeacon_secret_last_changed_timestamp_seconds`

Type: gauge

Last observed Secret metadata change timestamp as Unix seconds.

Labels: `cluster`, `namespace`, `secret_name`

```promql
time() - kbeacon_secret_last_changed_timestamp_seconds{cluster=~"$cluster",namespace=~"$namespace"}
```

This metric is emitted only for observed Secret objects.

### `kbeacon_secret_changes_total`

Type: counter

Observed Secret metadata update count during the current Agent lifecycle.

Labels: `cluster`, `namespace`, `secret_name`

```promql
increase(kbeacon_secret_changes_total{cluster=~"$cluster",namespace=~"$namespace"}[1h])
```

### `kbeacon_secret_info`

Type: gauge info

Secret metadata information. The value is always `1`.

Labels: `cluster`, `namespace`, `secret_name`, `type`, `owner_team`, `criticality`, `exists`

```promql
kbeacon_secret_info{cluster=~"$cluster",criticality=~"high|critical"}
```

### `kbeacon_unresolved_secret_references`

Type: gauge

Number of unresolved references by Secret.

Labels: `cluster`, `namespace`, `secret_name`

```promql
kbeacon_unresolved_secret_references{cluster=~"$cluster"} > 0
```

Unresolved references represent missing Secrets or Secrets that are unobservable because Secret object watching is disabled.

## Runtime metrics

### `kbeacon_build_info`

Type: gauge info

Build metadata. The value is always `1`.

Labels: `version`, `commit`, `go_version`

### `kbeacon_agent_info`

Type: gauge info

Agent runtime metadata. The value is always `1`.

Labels: `cluster`, `version`, `commit`

### `kbeacon_cache_sync_status`

Type: gauge

Kubernetes informer cache sync status. `1` means synced, `0` means not synced.

Labels: `cluster`, `resource`

```promql
min by (cluster, resource) (kbeacon_cache_sync_status{cluster=~"$cluster"})
```

Disabled resources are not emitted in this metric.

### `kbeacon_cache_objects`

Type: gauge

Current object count in the KBeacon graph cache.

Labels: `cluster`, `resource`

Known resource values include `Secret`, `Workload`, and `DependencyEdge`.

### `kbeacon_kubernetes_watch_events_total`

Type: counter

Kubernetes informer events observed by KBeacon.

Labels: `cluster`, `resource`, `event`

Known event values include `add`, `update`, and `delete`.

```promql
sum by (cluster, resource, event) (
  rate(kbeacon_kubernetes_watch_events_total{cluster=~"$cluster"}[5m])
)
```

### `kbeacon_graph_update_duration_seconds`

Type: histogram

Duration of dependency graph rebuilds.

Labels: `cluster`, `reason`

Known reason values include `initial-sync`, `add`, `update`, and `delete`.

```promql
histogram_quantile(
  0.95,
  sum by (cluster, le) (
    rate(kbeacon_graph_update_duration_seconds_bucket{cluster=~"$cluster"}[5m])
  )
)
```

## Low-privilege behavior

When `resourcesToWatch.core.secrets=false`, KBeacon does not observe Kubernetes Secret objects.

In this profile:

- workload-to-Secret references are still discovered from workload specs and annotations;
- `kbeacon_cluster_secret_count` reports observed and referenced graph Secret nodes, but Secret metadata is limited;
- referenced Secret series use `exists="false"` when the Secret object is unobservable;
- dependency edge series use `resolved="false"`;
- `kbeacon_secret_last_changed_timestamp_seconds` is not emitted for unobserved Secrets;
- Secret type and Secret annotation metadata are unavailable.

Treat `exists="false"` and `resolved="false"` as missing or unobservable in low-privilege environments.

## Edge metric cardinality

`kbeacon_dependency_edges` is optional because it includes workload and Secret names as labels.

```yaml
metrics:
  edge:
    enabled: false
```

When disabled:

- `kbeacon_dependency_edges` is not emitted;
- aggregate metrics remain available;
- the read-only Agent API remains available for edge-level inspection;
- Grafana Node Graph panels and edge detail tables are empty.

Keep edge metrics enabled when graph exploration is required. Disable them when Prometheus cardinality budgets are more important than edge-level visualization.

## Node Graph dashboard usage

Grafana Node Graph dashboards require `kbeacon_dependency_edges`.

The built-in graph dashboards use this metric to construct:

- workload nodes from `workload_kind`, `workload_namespace`, and `workload_name`;
- Secret nodes from `secret_namespace` and `secret_name`;
- edge identifiers from workload-to-Secret pairs;
- edge details from `discovery_mode`, `resolved`, `owner_team`, and `criticality`.

If `metrics.edge.enabled=false`, the dashboards can still load, but edge-level graph panels do not show dependency data.

## Security considerations

KBeacon metrics do not expose Kubernetes Secret values.

Metric labels may expose sensitive operational metadata:

- Secret names;
- workload names;
- namespace names;
- owner teams;
- criticality labels;
- unresolved or unobservable dependency state.

Protect Prometheus, Grafana Mimir, Grafana dashboards, exported metrics, and alert payloads according to the security model used for production operational metadata.

## Metrics intentionally not emitted

KBeacon intentionally avoids unbounded or high-risk labels such as:

- Secret key;
- environment variable name;
- full dependency source path;
- container name;
- Pod UID;
- Kubernetes `resourceVersion`;
- arbitrary Kubernetes labels;
- arbitrary Kubernetes annotations;
- raw error message text.

Use the read-only Agent API for detailed dependency source paths and Secret key-level inspection.

## Related documentation

- Dashboard queries: `docs/user-guide/dashboard-queries.md`
- Prometheus operations: `docs/operations/prometheus.md`
- Helm reference: `docs/reference/helm.md`
- API contract: `docs/api/openapi.yaml`

## Projected Secret volume source

Dependency edges discovered from projected Secret volumes use source type `volumes.projected.sources.secret`.

## cert-manager Certificate metrics behavior

cert-manager `Certificate` resources are normalized as Secret-consuming graph nodes when `resourcesToWatch.certManager.certificates=true`. Their dependency edges use source type `cert-manager.certificate.spec.secretName` and contribute to the same graph, impact, and dependency metrics as workload edges.

## ExternalSecret metrics behavior

External Secrets Operator `ExternalSecret` resources are normalized as Secret-consuming graph nodes when `resourcesToWatch.externalSecrets.externalSecrets=true`.

Their dependency edges use source type `external-secrets.externalsecret.spec.target.name` and contribute to the same graph, impact, dependency-map, workload dependency, and edge metrics as workload edges.

If the target Kubernetes Secret is observed by KBeacon, the edge is marked `resolved=true`. If Secret watching is disabled or the target Secret does not exist in the observed cache, the referenced Secret is represented with `exists=false` and the edge is marked `resolved=false`.

The `ExternalSecret` object name and namespace may appear in workload labels such as `workload_kind`, `workload_namespace`, and `workload_name` when edge metrics are enabled.

## SecretProviderClass metrics behavior

Secrets Store CSI Driver `SecretProviderClass` resources are normalized as Secret-consuming graph nodes when `resourcesToWatch.secretsStore.secretProviderClasses=true`.

Their dependency edges use source type `secrets-store.csi.secretproviderclass.spec.secretObjects.secretName` and contribute to the same graph, impact, dependency-map, workload dependency, and edge metrics as workload edges.

If the synced Kubernetes Secret is observed by KBeacon, the edge is marked `resolved=true`. If Secret watching is disabled or the target Secret does not exist in the observed cache, the referenced Secret is represented with `exists=false` and the edge is marked `resolved=false`.

The `SecretProviderClass` object name and namespace may appear in workload labels such as `workload_kind`, `workload_namespace`, and `workload_name` when edge metrics are enabled.

## Kafka connector metrics behavior

Strimzi `KafkaConnector` resources are normalized as Secret-consuming graph nodes when `resourcesToWatch.strimzi.kafkaConnectors=true`.

Confluent for Kubernetes `Connector` resources are normalized as Secret-consuming graph nodes when `resourcesToWatch.confluent.connectors=true`.

Kafka connector dependency edges contribute to the same graph, impact, dependency-map, workload dependency, and edge metrics as workload edges.

Strimzi inferred edges use source type `strimzi.kafkaconnector.spec.config.secrets`.

Confluent Connect REST authentication edges use source type `confluent.connector.spec.connectRest.authentication.secretRef`.

Confluent mounted Secret file edges use source type `confluent.connector.spec.configs.file.mountedSecret`.

If the referenced Kubernetes Secret is observed by KBeacon, the edge is marked `resolved=true`. If Secret watching is disabled or the target Secret does not exist in the observed cache, the referenced Secret is represented with `exists=false` and the edge is marked `resolved=false`.

`KafkaConnector` and `Connector` object names and namespaces may appear in workload labels such as `workload_kind`, `workload_namespace`, and `workload_name` when edge metrics are enabled.

ReplicaSets are used only as an owner-resolution cache. They may appear in readiness or runtime cache-sync status when `resourcesToWatch.apps.replicaSets=true`, but they are not emitted as workload nodes and do not appear as `workload_kind="ReplicaSet"` in dependency graph metrics.

## Prometheus/Mimir timeline recording rules

The Agent does not emit the following metric names directly. They are recording rules defined in `examples/prometheus/rules.yaml` for Prometheus or Grafana Mimir ruler.

| Recording rule | Labels | Value |
| --- | --- | --- |
| `kbeacon:secret_changes:increase_1h` | `cluster`, `namespace`, `secret_name` | Observed Secret update count over the last hour. |
| `kbeacon:secret_changes:increase_24h` | `cluster`, `namespace`, `secret_name` | Observed Secret update count over the last day. |
| `kbeacon:secret_age_since_change_seconds` | `cluster`, `namespace`, `secret_name` | Seconds since the last observed Secret update. |
| `kbeacon:recently_changed_affected_secrets` | `cluster`, `namespace`, `secret_name`, `owner_team`, `criticality`, `exists` | Affected workload count for Secrets changed within the last hour. |

These rules support historical dashboard views without adding storage to KBeacon. They rely on Prometheus or Mimir retention and should be adjusted if your production rule interval or lookback windows differ.

### Dependency edge timeline recording rules

The example Prometheus rule pack defines aggregate dependency edge timeline rules. The Agent does not emit these names directly.

| Recording rule | Source metric | Notes |
| --- | --- | --- |
| `kbeacon:dependency_edges:sum_by_workload_namespace` | `kbeacon_dependency_edges` | Requires edge metrics. Groups by `cluster` and `workload_namespace`. |
| `kbeacon:dependency_edges:sum_by_secret_namespace` | `kbeacon_dependency_edges` | Requires edge metrics. Groups by `cluster` and `secret_namespace`. |
| `kbeacon:dependency_edges:sum_by_owner_team` | `kbeacon_dependency_edges` | Requires edge metrics. Groups by `cluster` and `owner_team`. |
| `kbeacon:dependency_edges:sum_by_discovery_mode` | `kbeacon_dependency_edges` | Requires edge metrics. Groups by `cluster`, `discovery_mode`, and `resolved`. |
| `kbeacon:dependency_edges:changes_1h` | `kbeacon_cluster_dependency_count` | Counts aggregate cluster edge-count value changes over one hour. |
| `kbeacon:dependency_edges:net_delta_1h` | `kbeacon_cluster_dependency_count` | Shows net aggregate cluster edge-count delta over one hour. |

These are aggregate Prometheus or Mimir timelines. They are not an exact event log of added and removed dependency edges.
