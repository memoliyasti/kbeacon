# KBeacon

[![CI](https://github.com/memoliyasti/kbeacon/actions/workflows/ci.yaml/badge.svg)](https://github.com/memoliyasti/kbeacon/actions/workflows/ci.yaml)
[![Release](https://github.com/memoliyasti/kbeacon/actions/workflows/release.yaml/badge.svg)](https://github.com/memoliyasti/kbeacon/actions/workflows/release.yaml)
[![Docs](https://github.com/memoliyasti/kbeacon/actions/workflows/pages.yaml/badge.svg)](https://github.com/memoliyasti/kbeacon/actions/workflows/pages.yaml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

KBeacon is a lightweight Kubernetes Secret dependency intelligence agent for platform, SRE, and security teams.

It answers a practical operational question:

> If this Secret changes, which workloads are affected?

KBeacon watches Kubernetes resources with read-only access, builds an in-memory dependency graph, exports Prometheus metrics, and ships Grafana dashboards for blast-radius analysis.

KBeacon never exports Kubernetes Secret values. It uses Secret names, namespaces, metadata, and workload references to model dependency impact.

## Highlights

- Kubernetes-native discovery with `client-go` shared informers.
- Workload-to-Secret dependency extraction from standard Pod spec fields.
- Explicit dependency annotations for references that are not visible in Pod specs.
- `infer`, `explicit`, `hybrid`, and `disabled` discovery modes.
- Workload ownership and classification from KBeacon annotations or existing Kubernetes labels.
- Namespace include and exclude filtering.
- Low-privilege mode without Kubernetes Secret object reads.
- Read-only Agent API with filtering and bounded pagination.
- Prometheus metrics with an optional edge-cardinality guard.
- Grafana dashboards, including a Dependency Graph Explorer Node Graph dashboard.
- Demo manifests, alert rules, dashboard validation, and scale benchmark tooling.
- Helm chart and multi-arch GHCR images.

## How it works

```text
Kubernetes API
    |
    | read-only watch/list
    v
KBeacon Agent
    |
    | /metrics
    v
Prometheus or Grafana Mimir
    |
    | PromQL
    v
Grafana dashboards and alerting
```

The Agent keeps only the current dependency graph in memory. Prometheus stores metrics history. Grafana renders the operational view.

## Install with Helm

The default image is published from this repository to GHCR.

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set image.repository=ghcr.io/memoliyasti/kbeacon \
  --set image.tag=0.2.3
```

The GHCR package for this project is intended to be public, so the default install does not require an image pull Secret.

## Verify the Agent

```bash
kubectl -n kbeacon-system rollout status deploy/kbeacon
kubectl -n kbeacon-system logs deploy/kbeacon --tail=100
kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
```

```bash
curl -sS http://127.0.0.1:8081/readyz | jq
curl -sS http://127.0.0.1:8081/api/v1/config | jq
curl -sS http://127.0.0.1:8081/api/v1/secrets | jq
curl -sS http://127.0.0.1:8081/api/v1/workloads | jq
```

## Discovery modes

| Mode | Behavior |
| --- | --- |
| `infer` | Discover Secret references from Pod specs. |
| `explicit` | Use only `kbeacon.io/watch-secrets` and `kbeacon.io/watch-secrets-json`. |
| `hybrid` | Combine inferred and explicit dependencies. This is the recommended default. |
| `disabled` | Ignore the workload. |

Implemented inferred sources:

- `env.valueFrom.secretKeyRef`
- `envFrom.secretRef`
- `volumes.secret`
- `imagePullSecrets`

Explicit references use this grammar:

```text
secret
secret#key
namespace/secret
namespace/secret#key
```

## Low-privilege mode

Some clusters do not allow observability agents to read Kubernetes Secret objects. KBeacon can still discover workload references without Secret object access.

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set resourcesToWatch.core.secrets=false
```

In this mode, referenced Secrets are represented as `exists=false`, dependency edges are marked `resolved=false`, and Secret type or change metadata is unavailable.

## Metrics cardinality guard

`kbeacon_dependency_edges` is the detailed edge metric. It includes workload and Secret names as labels and powers the graph dashboards.

Disable detailed edge metrics when cardinality is more important than edge-level graph panels:

```yaml
metrics:
  edge:
    enabled: false
```

Aggregate metrics and the Agent API remain available.

## Grafana dashboards

KBeacon ships dashboard JSON in two locations:

- `dashboards/`
- `charts/kbeacon/dashboards/`

Enable dashboard ConfigMaps with Helm:

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --set cluster.name=prod-eu-1 \
  --set dashboards.enabled=true
```

Included dashboards:

- `KBeacon / Cluster Overview`
- `KBeacon / Secret Dependency Map`
- `KBeacon / Team Overview`
- `KBeacon / Dependency Graph Explorer`

The Dependency Graph Explorer uses Grafana Node Graph and requires `metrics.edge.enabled=true`.

## Prometheus integration

Prometheus Operator users can enable a ServiceMonitor:

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set serviceMonitor.enabled=true \
  --set serviceMonitor.labels.release=kube-prometheus-stack
```

Clusters that use annotation-based scraping can enable Service annotations:

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set prometheus.scrapeAnnotations.enabled=true
```

## Blast-radius demo

A runnable demo is available in `examples/demo-blast-radius`.

```bash
./examples/demo-blast-radius/run.sh apply
kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
curl -sS http://127.0.0.1:8081/api/v1/secrets/payments/payments-db/impact | jq ".data.summary"
```

The demo creates a small multi-namespace graph with inferred, explicit, hybrid, and unresolved Secret references.

## Local development

Minikube support is kept for local development and end-to-end smoke testing.

```bash
./hack/local-dev/deploy-incluster-minikube.sh
./hack/local-dev/configure-prometheus-incluster.sh
./hack/local-dev/smoke-incluster.sh
```

The local workflow exercises Helm, RBAC, Service networking, Kubernetes informers, Prometheus scraping, Grafana dashboards, and the Agent API.

## Scale benchmark

Generate deterministic scale fixtures:

```bash
make scale-lint
```

Run a live benchmark against a reachable local Agent API:

```bash
make scale-benchmark
```

Reports are written under `/tmp/kbeacon-scale-benchmark` by default.

## Documentation

- Website: https://memoliyasti.github.io/kbeacon/
- Getting started: `docs/getting-started.md`
- Installation: `docs/user-guide/installation.md`
- Configuration: `docs/user-guide/configuration.md`
- Discovery modes: `docs/user-guide/discovery-modes.md`
- Blast-radius demo: `docs/user-guide/blast-radius-demo.md`
- Dashboards: `docs/user-guide/dashboards.md`
- Dashboard queries: `docs/user-guide/dashboard-queries.md`
- Scale testing: `docs/user-guide/scale-testing.md`
- Helm reference: `docs/reference/helm.md`
- Metrics reference: `docs/reference/metrics.md`
- Annotation reference: `docs/reference/annotations.md`
- Agent API: `docs/api/openapi.yaml`

## Development checks

```bash
make validate-ci
```

This runs Go formatting, tests, build, Helm rendering, Prometheus rule validation, dashboard validation, demo linting, scale fixture checks, stale checks, and release metadata checks.

## Releases

Release tags use semantic versioning.

```bash
git tag -a v0.2.3 -m "KBeacon v0.2.3"
git push origin v0.2.3
```

The release workflow publishes GitHub Release assets, Linux and macOS binaries, a Helm chart package, SHA256 checksums, and multi-arch container images for `linux/amd64` and `linux/arm64`.

## Community

- Contributing: `CONTRIBUTING.md`
- Code of Conduct: `CODE_OF_CONDUCT.md`
- Security policy: `SECURITY.md`
- Support: `SUPPORT.md`
- Governance: `GOVERNANCE.md`
- Maintainers: `MAINTAINERS.md`
- Adopters: `ADOPTERS.md`

## License

Apache License 2.0. See `LICENSE` and `NOTICE`.
