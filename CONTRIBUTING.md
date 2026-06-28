# Contributing to KBeacon

Thank you for contributing to KBeacon.

## Development workflow

1. Open an issue for non-trivial changes.
2. Keep changes focused.
3. Add tests for discovery, graph behavior, metrics, or API changes.
4. Update documentation when user-facing behavior changes.
5. Run validation locally before opening a pull request.

Validation:

    go fmt ./...
    go test ./...
    go build -o ./bin/kbeacon-agent ./cmd/kbeacon-agent
    helm lint ./charts/kbeacon --set cluster.name=ci
    helm template kbeacon ./charts/kbeacon --namespace kbeacon-system --set cluster.name=ci --set dashboards.enabled=true > /tmp/kbeacon-rendered.yaml
    docker run --rm -i --entrypoint=promtool prom/prometheus:v3.1.0 check rules /dev/stdin < examples/prometheus/rules.yaml

## Contribution rules

- Do not export Secret values.
- Keep metric cardinality bounded.
- Keep Kubernetes RBAC read-only.
- Prefer informer-based discovery over polling.
- Keep the default deployment simple.

## Developer Certificate of Origin

By contributing, you certify that you have the right to submit the contribution under the project license.
