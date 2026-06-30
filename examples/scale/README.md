# Scale fixtures and benchmarks

This directory documents KBeacon scale testing helpers.

## Deterministic fixture generator

```bash
./hack/generate-scale-fixture.sh /tmp/kbeacon-scale-fixture kbeacon-scale 25 100
```

The generator writes Kubernetes manifests and an `expected-summary.json` file. It is useful for dry-run validation and for creating repeatable live workloads.

## Live benchmark report

Run against a live KBeacon Agent API:

```bash
kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
make scale-benchmark
```

For larger levels:

```bash
KBEACON_SCALE_LEVELS="100 1000 5000" make scale-benchmark
```

Reports are written to `/tmp/kbeacon-scale-benchmark/reports/<timestamp>/` by default and are not committed to the repository.
