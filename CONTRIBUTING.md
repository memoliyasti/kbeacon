# Contributing to KBeacon

Thank you for contributing to KBeacon.

KBeacon is a lightweight Kubernetes-native Secret Dependency Intelligence Agent. Contributions should preserve the core principles:

- read-only Kubernetes access;
- no Secret value export;
- Prometheus/Mimir for time series storage;
- Grafana dashboards and alerting for UI workflows;
- bounded metric cardinality;
- small operational footprint.

## Ways to contribute

You can help by:

- reporting reproducible bugs;
- improving docs and examples;
- adding tests;
- improving Helm chart safety;
- improving dashboards and alert rules;
- implementing new dependency extractors;
- improving performance and reliability.

## Before opening a pull request

For non-trivial changes, open an issue first so the design can be discussed.

Run the local validation suite:

    go fmt ./...
    go test ./...
    go build -o ./bin/kbeacon-agent ./cmd/kbeacon-agent
    helm lint ./charts/kbeacon --set cluster.name=ci
    helm template kbeacon ./charts/kbeacon --namespace kbeacon-system --set cluster.name=ci --set dashboards.enabled=true > /tmp/kbeacon-rendered.yaml
    docker run --rm -i --entrypoint=promtool prom/prometheus:v3.1.0 check rules /dev/stdin < examples/prometheus/rules.yaml

For documentation changes, also run:

    python3 -m pip install -r requirements-docs.txt
    mkdocs build --strict

## Development workflow

1. Fork the repository.
2. Create a branch from `main`.
3. Make small, reviewable commits.
4. Add or update tests.
5. Update documentation when behavior changes.
6. Open a pull request using the PR template.

## Metrics contribution rules

Prometheus metrics are a public contract. New metrics or labels must be documented in `docs/reference/metrics.md`.

Avoid labels with unbounded cardinality:

- Secret keys;
- environment variable names;
- full source paths;
- container names;
- Pod UIDs;
- Kubernetes resource versions;
- arbitrary user labels or annotations;
- raw error messages.

Use the HTTP API for detailed inspection data.

## Security contribution rules

KBeacon must never expose Kubernetes Secret `data` or `stringData`.

Do not include real tokens, kubeconfigs, registry credentials, customer names, or Secret values in issues, pull requests, tests, docs, or examples.

## Adding Kubernetes resource support

New resource support should be implemented as an extractor and should include:

- unit tests;
- documentation;
- RBAC review;
- metric cardinality review;
- examples when applicable.

## Commit style

Use concise conventional-style prefixes when possible:

- `feat:`
- `fix:`
- `docs:`
- `test:`
- `chore:`
- `refactor:`
- `ci:`

## Certificate of Origin

By contributing to KBeacon, you certify that you have the right to submit your contribution under the Apache License 2.0.
