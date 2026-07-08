# Dashboards

KBeacon ships Grafana dashboard JSON for Prometheus-compatible metrics.

The dashboards are designed for platform, SRE, DevOps, and security engineering workflows where teams need to understand Kubernetes Secret blast radius without exposing Secret values.

## Dashboard sources

Dashboard JSON is maintained in two locations.

| Path | Purpose |
| --- | --- |
| `dashboards/` | Standalone dashboard JSON for review, import, and development. |
| `charts/kbeacon/dashboards/` | Dashboard JSON packaged into the Helm chart. |

The repository keeps these copies aligned through dashboard validation checks.

## Helm rendering model

Dashboard ConfigMaps are optional.

```yaml
dashboards:
  enabled: false
  labels:
    grafana_dashboard: "1"
  annotations: {}
```

Enable dashboard rendering only when Grafana is configured to discover dashboard ConfigMaps by label.

The chart renders dashboard JSON from `charts/kbeacon/dashboards/` into a ConfigMap when dashboard rendering is enabled.

## Included dashboards

| Dashboard | File | Primary purpose |
| --- | --- | --- |
| `KBeacon / Cluster Overview` | `kbeacon-cluster-overview.json` | Cluster-level graph size, scrape health, runtime health, and update activity. |
| `KBeacon / Secret Dependency Map` | `kbeacon-secret-dependency-map.json` | Secret impact, affected workloads, unresolved references, and edge-level dependency details. |
| `KBeacon / Team Overview` | `kbeacon-team-overview.json` | Owner-team oriented views for affected Secrets, workloads, and criticality. |
| `KBeacon / Dependency Graph Explorer` | `kbeacon-dependency-graph-explorer.json` | Interactive workload-to-Secret Node Graph exploration. |

## Data source expectations

Dashboards expect KBeacon metrics in Prometheus or Grafana Mimir.

KBeacon domain metrics always include a `cluster` label. Prometheus scrape labels such as `job` and `instance` depend on the selected scrape integration profile.

Dashboard queries should use variables instead of assuming one hard-coded scrape job.

Common variables:

| Variable | Purpose |
| --- | --- |
| `$datasource` | Grafana Prometheus-compatible datasource. |
| `$job` | Prometheus scrape job selector. |
| `$cluster` | KBeacon logical cluster selector. |
| `$namespace` | Workload or Secret namespace selector, depending on panel context. |
| `$owner_team` | Owner team selector. |
| `$criticality` | Criticality selector. |
| `$resolved` | Dependency resolution selector for edge views. |
| `$discovery_mode` | Discovery mode selector for edge views. |

## Dependency Graph Explorer dashboard

The standalone `KBeacon / Dependency Graph Explorer` dashboard provides an interactive Grafana Node Graph view of workload-to-Secret dependencies.

It is intended for investigation workflows such as:

- understanding which workloads depend on a selected Secret namespace;
- exploring cross-namespace Secret references;
- identifying unresolved or unobservable references;
- filtering graph edges by owner team, criticality, discovery mode, and resolution state;
- reviewing the raw edge detail table behind the graph.

The dashboard includes:

- summary stat panels for active edges, unresolved references, high or critical Secrets, and maximum impact score;
- a large Node Graph panel built from `kbeacon_dependency_edges`;
- tables for high-impact Secrets, unresolved references, and edge details.

The Node Graph panel requires `metrics.edge.enabled=true`.

## Node Graph data model

Grafana Node Graph panels need edge fields such as `source`, `target`, and `id`.

KBeacon dashboards derive those fields from `kbeacon_dependency_edges` using PromQL label joins.

The graph model is:

| Node or edge field | Source labels | Meaning |
| --- | --- | --- |
| `source` | `workload_kind`, `workload_namespace`, `workload_name` | Workload node identifier. |
| `target` | `secret_namespace`, `secret_name` | Secret node identifier. |
| `id` | `source`, `target` | Unique edge identifier. |
| `mainstat` | `discovery_mode`, `resolved` | Compact edge status. |
| `detail__owner_team` | `owner_team` | Edge owner team detail. |
| `detail__criticality` | `criticality` | Edge criticality detail. |

If `metrics.edge.enabled=false`, the Node Graph panel and edge detail table do not show edge-level data. Aggregate dashboard panels remain available.

## Cardinality guidance

`kbeacon_dependency_edges` includes workload and Secret names as labels. This is useful for graph exploration and incident analysis, but it can create many time series in large clusters.

Recommended approach:

- keep edge metrics enabled for development, staging, and small or medium clusters;
- use namespace and owner-team filters when exploring large graphs;
- disable edge metrics when Prometheus cardinality budget is more important than edge-level visualization;
- rely on aggregate metrics and the Agent API when edge metrics are disabled.

## Security considerations

Dashboard panels do not show Kubernetes Secret values.

They can still expose sensitive operational metadata:

- Secret names;
- workload names;
- namespace names;
- owner teams;
- criticality labels;
- unresolved reference state.

Protect Grafana dashboard access according to the same policy used for production observability metadata.

## Dashboard validation

Dashboard JSON is validated by the repository dashboard validator.

The validator checks that:

- dashboard files are valid JSON objects;
- dashboards reference KBeacon metrics;
- packaged chart dashboard copies match root dashboard copies where required;
- the Secret Dependency Map includes a Node Graph panel;
- the Dependency Graph Explorer dashboard exists and contains the expected Node Graph query fields.

The repository validation entry point is:

```bash
make validate-ci
```

## Related documentation

- Dashboard PromQL snippets: `docs/user-guide/dashboard-queries.md`
- Metrics reference: `docs/reference/metrics.md`
- Prometheus operations: `docs/operations/prometheus.md`
- Helm reference: `docs/reference/helm.md`

## Dashboard data links

The dashboards include Grafana data links to the read-only KBeacon Agent API.

Set the `kbeacon_api_url` dashboard variable to the reachable Agent API base URL for your environment.

For kube-native CLI workflows, use `kbeacon` from a shell with kubeconfig access. Grafana API data links still require a browser-reachable Agent API URL when you enable them; set `kbeacon_api_url` to an approved internal endpoint for that environment.

The links open read-only API views such as Secret impact, Workload dependencies, and filtered dependency-map results.

They use existing KBeacon metric labels and do not read, log, export, or expose Kubernetes Secret values.
