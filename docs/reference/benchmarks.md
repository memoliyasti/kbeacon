# Benchmark baseline

KBeacon should stay lightweight. Use this page to track repeatable local and CI scale checks.

## Current local scale fixture

The repository includes a deterministic fixture generator.

```bash
make scale-lint
make scale-generate
```

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
