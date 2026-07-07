# KBeacon Grafana Dashboards

This directory contains starter Grafana dashboards for KBeacon.

| Dashboard | Purpose |
| --- | --- |
| `kbeacon-cluster-overview.json` | Cluster health, dependency counts, cache status, and watcher errors. |
| `kbeacon-secret-dependency-map.json` | Top affected Secrets and dependency edge exploration. |
| `kbeacon-team-overview.json` | Team ownership, dependency counts, and high-impact Secrets. |

Import them into Grafana with a Prometheus-compatible Mimir data source. The dashboards use a `${datasource}` variable and standard KBeacon metric names.

## Dashboard data links

The dashboards include Grafana data links to the read-only KBeacon Agent API.

Set the `kbeacon_api_url` dashboard variable to the reachable Agent API base URL for your environment.

For local port-forward workflows, the default is usually `http://127.0.0.1:8081`.

The links open read-only API views such as Secret impact, Workload dependencies, and filtered dependency-map results.

They use existing KBeacon metric labels and do not read, log, export, or expose Kubernetes Secret values.
