# KBeacon Grafana Dashboards

This directory contains starter Grafana dashboards for KBeacon.

| Dashboard | Purpose |
| --- | --- |
| `kbeacon-cluster-overview.json` | Cluster health, dependency counts, cache status, and watcher errors. |
| `kbeacon-secret-dependency-map.json` | Top affected Secrets and dependency edge exploration. |
| `kbeacon-team-overview.json` | Team ownership, dependency counts, and high-impact Secrets. |

Import them into Grafana with a Prometheus-compatible Mimir data source. The dashboards use a `${datasource}` variable and standard KBeacon metric names.
