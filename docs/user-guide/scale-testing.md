# Scale testing

KBeacon includes deterministic scale tooling for two separate purposes:

1. fixture generation for repeatable manifests;
2. live benchmark reporting against a running Agent.

The scale tools are intentionally developer utilities. They are not run as part of normal CI because large fixture levels can create many Kubernetes objects.

## Generate a fixture

Generate 25 Secrets and 100 workloads:

```bash
./hack/generate-scale-fixture.sh /tmp/kbeacon-scale-fixture kbeacon-scale 25 100
```

The generator writes:

- `namespace.yaml`
- `secrets.yaml`
- `workloads.yaml`
- `expected-summary.json`

Dry-run the generated manifests:

```bash
make scale-dry-run
```

## Live benchmark report

The live benchmark requires a running KBeacon Agent and a reachable local Agent API.

Start a port-forward:

```bash
kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
```

Run a small benchmark:

```bash
make scale-benchmark
```

Run explicit levels, for example 100, 1k, and 5k workloads:

```bash
KBEACON_SCALE_LEVELS="100 1000 5000" make scale-benchmark
```

The benchmark harness:

- generates one namespace per level;
- applies deterministic Secret and workload manifests;
- waits until `/api/v1/workloads?namespace=<level>` reports the expected workload count;
- measures API response time for `/api/v1/config`, `/api/v1/secrets`, `/api/v1/workloads`, and `/api/v1/dependency-map`;
- counts emitted `/metrics` samples;
- records graph rebuild average from direct metrics;
- records Prometheus p95 graph rebuild latency when `PROMETHEUS_URL` is reachable;
- records best-effort `kubectl top pod` memory when metrics-server is available;
- writes JSON and Markdown reports under `/tmp/kbeacon-scale-benchmark/reports/<timestamp>/`.

Useful environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `KBEACON_URL` | `http://127.0.0.1:8081` | Local Agent API URL. |
| `PROMETHEUS_URL` | `http://127.0.0.1:9090` | Optional Prometheus API URL. |
| `KBEACON_SCALE_LEVELS` | `100 1000` | Space-separated workload counts. |
| `KBEACON_SCALE_SECRET_RATIO` | `4` | Approximate workloads-per-Secret ratio. |
| `KBEACON_SCALE_RETAIN` | `false` | Keep generated namespaces after each level. |
| `KBEACON_SCALE_BENCHMARK_OUT` | `/tmp/kbeacon-scale-benchmark` | Report output root. |

## Reading results

The Markdown summary contains one row per level:

- generated workload count;
- generated Secret count;
- expected edge count;
- observed workload count;
- observed dependency-map edge count;
- list and dependency-map API latency;
- metric sample count;
- graph rebuild average and optional Prometheus p95;
- optional pod memory.

Benchmark numbers depend on the Kubernetes runtime, node size, Prometheus scrape interval, and whether `metrics.edge.enabled` is enabled. Use the same cluster shape and values when comparing runs.
