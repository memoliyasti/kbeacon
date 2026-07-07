# Prometheus

KBeacon exposes Prometheus metrics from the Agent HTTP endpoint at `/metrics`.

Prometheus and Grafana Mimir are the recommended storage layer for KBeacon dependency intelligence. KBeacon keeps only the current dependency graph in memory and exports that state as metrics.

## Metrics endpoint

The chart exposes the Agent HTTP port through the KBeacon Service.

```yaml
service:
  type: ClusterIP
  port: 8080
```

The default metrics path is `/metrics`.

```yaml
agent:
  http:
    port: 8080
```

Health endpoints and the read-only Agent API use the same HTTP listener.

## Scrape integration profiles

KBeacon supports three common Prometheus integration patterns.

## Prometheus Operator ServiceMonitor

Use this profile when the cluster runs Prometheus Operator and watches ServiceMonitor resources.

```yaml
serviceMonitor:
  enabled: true
  labels:
    release: kube-prometheus-stack
  annotations: {}
  interval: 30s
  scrapeTimeout: 10s
  honorLabels: true
  metricRelabelings: []
  relabelings: []
```

`serviceMonitor.honorLabels=true` is the recommended default. It preserves KBeacon metric labels such as `namespace`, `secret_name`, and `workload_name` instead of allowing target labels to rewrite them.

## Service scrape annotations

Use this profile only when the Prometheus deployment discovers Services through `prometheus.io/*` annotations.

```yaml
prometheus:
  scrapeAnnotations:
    enabled: true
    target: service
    path: /metrics
    port: "8080"
```

This profile adds scrape annotations to the KBeacon Service. It does not add annotations to application workloads.

## Static scrape configuration

Use this profile when scrape targets are managed centrally.

```yaml
scrape_configs:
  - job_name: kbeacon-agent
    honor_labels: true
    metrics_path: /metrics
    static_configs:
      - targets:
          - kbeacon.kbeacon-system.svc.cluster.local:8080
        labels:
          cluster: prod-eu-1
          app: kbeacon
          component: agent
```

The `cluster` label should match `cluster.name` from the KBeacon values file.

## Label handling

Every KBeacon domain metric includes a `cluster` label.

Prometheus adds scrape labels such as `job` and `instance`. Their values depend on the scrape integration profile.

For portable dashboards and rules:

- filter by `cluster` for KBeacon domain identity;
- avoid hard-coding one scrape job name where multiple integration styles are supported;
- keep `honor_labels=true` when using Prometheus Operator or static scrape configs;
- protect Secret and workload name labels as sensitive metadata.

When `honorLabels` is disabled, Prometheus may rename conflicting labels to exported labels such as `exported_namespace`. Dashboard queries should be reviewed before disabling honor labels.

## Metric cardinality

`kbeacon_dependency_edges` is the highest-cardinality KBeacon metric family.

It includes workload and Secret names as labels and powers edge-level troubleshooting and Grafana Node Graph dashboards.

```yaml
metrics:
  edge:
    enabled: true
```

For large clusters or strict Prometheus cardinality budgets, disable detailed edge metrics.

```yaml
metrics:
  edge:
    enabled: false
```

When edge metrics are disabled:

- `kbeacon_dependency_edges` is not emitted;
- aggregate Secret and workload metrics remain available;
- the read-only Agent API still exposes dependency details;
- Grafana panels that depend on edge-level series are empty.

## Grafana Mimir

KBeacon does not require Mimir, but it works well with Prometheus `remote_write` into Mimir for multi-cluster history and central Grafana dashboards.

Recommended model:

- scrape one KBeacon Agent per cluster;
- keep a stable `cluster` label;
- remote-write KBeacon metrics with the rest of the platform metrics;
- build dashboards using `cluster`, `namespace`, `owner_team`, and `criticality` variables.

An example remote-write configuration is stored in `examples/prometheus/remote-write-mimir.yaml`.

## Historical timelines with Prometheus and Mimir

KBeacon keeps the current dependency graph in memory. Prometheus or Grafana Mimir provide historical retention by storing metric samples over time.

The example rule pack includes timeline-oriented recording rules:

| Recording rule | Meaning |
| --- | --- |
| `kbeacon:secret_changes:increase_1h` | Observed Secret metadata update count over the last hour, grouped by cluster, namespace, and Secret name. |
| `kbeacon:secret_changes:increase_24h` | Observed Secret metadata update count over the last day, grouped by cluster, namespace, and Secret name. |
| `kbeacon:secret_age_since_change_seconds` | Age in seconds since the last observed Secret update. |
| `kbeacon:recently_changed_affected_secrets` | Recently changed Secrets with affected workloads. The sample value is the affected workload count. |

This preserves KBeacon's lightweight model: no database is added to the Agent, and long-term history remains in Prometheus or Mimir.

Timeline views depend on scrape interval, rule interval, and retention. KBeacon does not reconstruct events before Prometheus retention, does not read Secret values, and does not claim that a metadata update is a payload-only Secret data change.

## Alerting and recording rules

Example Prometheus rules are stored in `examples/prometheus/rules.yaml`.

The rule set focuses on:

- unresolved Secret references;
- high-impact Secrets;
- broad Secret fan-out;
- Agent scrape health;
- graph update latency.

Rules should be reviewed against each cluster security and cardinality model before production use.

## Security considerations

KBeacon metrics do not expose Kubernetes Secret values.

Metrics can still include sensitive metadata:

- Secret names;
- workload names;
- namespaces;
- owner teams;
- criticality labels;
- dependency resolution state.

Protect Prometheus, Mimir, Grafana, and any exported metrics according to the same standard used for operational metadata in the cluster.

## Validation

Recommended validation checks:

- render the chart with the selected scrape profile;
- confirm low-privilege mode does not render Secret RBAC when enabled;
- validate Prometheus rules with `promtool`;
- validate dashboard JSON when dashboards are enabled;
- run the repository validation target with `make validate-ci`.

## Related documentation

- Metrics reference: `docs/reference/metrics.md`
- Helm reference: `docs/reference/helm.md`
- Dashboard guide: `docs/user-guide/dashboards.md`
- Dashboard queries: `docs/user-guide/dashboard-queries.md`
- Alerting guide: `docs/user-guide/alerting.md`

### Dependency edge timeline rules

The example recording rule pack includes aggregate dependency edge timeline rules:

| Rule | Purpose |
| --- | --- |
| `kbeacon:dependency_edges:sum_by_workload_namespace` | Current edge count grouped by workload namespace. |
| `kbeacon:dependency_edges:sum_by_secret_namespace` | Current edge count grouped by Secret namespace. |
| `kbeacon:dependency_edges:sum_by_owner_team` | Current edge count grouped by owner team. |
| `kbeacon:dependency_edges:sum_by_discovery_mode` | Current edge count grouped by discovery mode and resolution status. |
| `kbeacon:dependency_edges:changes_1h` | Number of aggregate cluster edge-count changes over one hour. |
| `kbeacon:dependency_edges:net_delta_1h` | Net aggregate cluster edge-count delta over one hour. |

The grouped rules depend on `kbeacon_dependency_edges`, so they require `metrics.edge.enabled=true`. The `changes_1h` and `net_delta_1h` rules use the aggregate cluster dependency count and remain lower-cardinality.
