# Contributing to KBeacon

Thanks for your interest in KBeacon.

KBeacon is intentionally small: one Agent, read-only Kubernetes access, Prometheus metrics, a read-only API, Grafana dashboards, and Helm packaging. Contributions should preserve that shape.

## Development principles

- Do not export Secret values.
- Keep Kubernetes permissions read-only.
- Keep metric labels bounded and documented.
- Prefer simple extractors over broad framework changes.
- Keep production behavior documented.
- Keep local development reproducible.

## Development workflow

1. Open an issue for non-trivial behavior changes.
2. Fork the repository or create a feature branch.
3. Add or update tests.
4. Update docs when public behavior changes.
5. Run the validation commands below.
6. Open a pull request with a clear summary.

## Validation

    go fmt ./...
    go test ./...
    go build -o ./bin/kbeacon-agent ./cmd/kbeacon-agent

    helm lint ./charts/kbeacon --set cluster.name=ci

    helm template kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --set cluster.name=ci \
      --set dashboards.enabled=true \
      > /tmp/kbeacon-rendered.yaml

    docker run --rm -i \
      --entrypoint=promtool \
      prom/prometheus:v3.1.0 \
      check rules /dev/stdin < examples/prometheus/rules.yaml

For documentation changes:

    python3 -m venv .venv-docs
    . .venv-docs/bin/activate
    python -m pip install -r requirements-docs.txt
    mkdocs build --strict
    deactivate

## Adding Kubernetes resource support

New resource support should be implemented through the discovery/extractor path and covered by tests. The pull request should document:

- resource kind and API version;
- supported Secret reference patterns;
- expected metric labels;
- RBAC requirements;
- behavior when the resource is disabled or unavailable.

## Metrics and API changes

Metrics and API responses are user-facing contracts. Changes should include:

- docs update;
- dashboard or rule update if needed;
- cardinality review;
- migration note if names or labels change.

## Security checklist

Before submitting, confirm:

- no Secret values are logged, exported, or stored;
- no new write permissions are introduced;
- no unbounded label values are added;
- no real tokens, kubeconfigs, or internal identifiers are committed.
