# Benchmark baseline

KBeacon should stay lightweight. Use this page to track repeatable local and CI scale checks.

## Current local scale fixture

The repository includes a deterministic fixture generator.

```bash
make scale-lint
make scale-generate
```

## Observed Minikube baseline

This baseline was captured on a local Minikube cluster after installing the current `main` Agent build and applying the deterministic small scale fixture.

Environment:

| Field | Value |
| --- | --- |
| Cluster | Minikube |
| Minikube resources | 4 CPUs, 8192 MiB memory |
| Agent image | `kbeacon-agent:local-version-visibility` |
| Agent version | `local-version-visibility` |
| Agent commit | `c1144bbcdce75b1075dc579ea32d6e2d96cb9035` |
| Fixture | `./hack/generate-scale-fixture.sh /tmp/kbeacon-benchmark-small kbeacon-benchmark-small 25 100` |
| Fixture expected edges | 300 |

Observed graph before fixture:

| Metric | Value |
| --- | ---: |
| API graph edges | 17 |
| API graph secrets | 27 |
| API graph workloads | 17 |
| Pod CPU | 1m |
| Pod memory | 21Mi |

Observed graph after fixture:

| Metric | Value |
| --- | ---: |
| API graph edges | 317 |
| API graph secrets | 52 |
| API graph workloads | 117 |
| Dependency map edges returned | 317 |
| Prometheus `kbeacon_cluster_dependency_count` | 317 |
| Prometheus `kbeacon_cluster_secret_count` | 51 |
| Prometheus `kbeacon_cluster_workload_count` | 117 |
| `/metrics` response size | 192,925 bytes |
| Pod CPU | 15m |
| Pod memory | 32Mi |
| Pod restarts | 0 |

CLI/API timings through the kube-native Service proxy:

| Command | Observed wall time |
| --- | ---: |
| `kbeacon ready` | 0.02s |
| `kbeacon get config` | 0.02s |
| `kbeacon get secrets --limit 500` | 0.02s |
| `kbeacon get workloads --limit 500` | 0.02s |
| `kbeacon get dependency-map --limit 5000` | 0.02s |

Notes:

- The baseline is intentionally small and repeatable. It is suitable for local regression comparison, not for capacity planning.
- API graph Secret count and Prometheus `kbeacon_cluster_secret_count` are recorded separately because the API graph includes current graph references while the metric reports the collector's Secret count view at scrape time.
- Large 1k/10k workload tiers should be run only when Docker disk and Minikube resources are healthy.

## Suggested benchmark tiers

| Tier | Secrets | Workloads | Expected purpose |
| --- | ---: | ---: | --- |
| small | 25 | 100 | Fast local smoke and PR sanity. |
| medium | 250 | 1000 | Release candidate validation. |
| large | 1000 | 10000 | Manual performance baseline before major metric or graph changes. |

## Metrics to capture

- Agent memory and CPU from `kubectl top pod`.
- `/metrics` response size.
- `kbeacon_graph_update_duration_seconds` p95.
- `kbeacon_cluster_dependency_count`.
- API latency for `/api/v1/config`, `/api/v1/secrets`, `/api/v1/workloads`, and `/api/v1/dependency-map`.

## Manual baseline command

```bash
./hack/benchmark-scale.sh
```

Record baseline changes in release notes when graph, metrics, or discovery behavior changes significantly.
