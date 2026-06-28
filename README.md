# KBeacon

[![CI](https://github.com/memoliyasti/kbeacon/actions/workflows/ci.yaml/badge.svg)](https://github.com/memoliyasti/kbeacon/actions/workflows/ci.yaml)
[![Release](https://github.com/memoliyasti/kbeacon/actions/workflows/release.yaml/badge.svg)](https://github.com/memoliyasti/kbeacon/actions/workflows/release.yaml)
[![Docs](https://github.com/memoliyasti/kbeacon/actions/workflows/pages.yaml/badge.svg)](https://github.com/memoliyasti/kbeacon/actions/workflows/pages.yaml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

KBeacon is a lightweight Kubernetes-native Secret dependency intelligence agent.

It answers one operational question:

> If this Secret changes, what workloads are affected?

KBeacon watches Kubernetes resources with read-only access, builds an in-memory dependency graph, exposes Prometheus metrics, and provides a small read-only Agent API. It is designed to work with the observability stack platform teams already run: Prometheus, Grafana, and optionally Grafana Mimir.

## Why KBeacon?

Secret rotations, certificate renewals, registry credential updates, and database password changes can affect many workloads. Without dependency intelligence, teams often rely on manual kubectl searches, manifest grep, tribal knowledge, or incident response after a change already broke something.

KBeacon gives platform and SRE teams a current, queryable view of:

- which workloads reference each Secret;
- which Secrets have large fan-out;
- which teams and namespaces are affected;
- which workloads reference missing or unobservable Secrets;
- which Secret changes should be reviewed before rollout.


## Project positioning

KBeacon is not a replacement for Kubernetes, Prometheus, Grafana, or Mimir.

Kubernetes already stores the workload specs. Prometheus and Grafana already provide metrics, dashboards, and alerting. The missing layer is a continuously updated, normalized dependency graph that answers which workloads depend on which Secrets.

KBeacon is that discovery layer.

It uses the Kubernetes API as source of truth, keeps only an in-memory current graph, exposes Prometheus metrics and a read-only API, and leaves storage and visualization to the observability stack.

See [Why KBeacon?](docs/concepts/why-kbeacon.md) for the detailed project boundary.

## Current status

KBeacon is early-stage but functional. The current implementation includes:

- Kubernetes client-go in-cluster and local kubeconfig support.
- Shared informer based discovery.
- Watchers for Secret, Pod, Deployment, StatefulSet, DaemonSet, Job, and CronJob.
- Dependency extraction from env.valueFrom.secretKeyRef.
- Dependency extraction from envFrom.secretRef.
- Dependency extraction from volumes.secret.
- Dependency extraction from imagePullSecrets.
- Explicit dependencies through kbeacon.io/watch-secrets.
- Explicit dependencies through kbeacon.io/watch-secrets-json.
- Namespace include and exclude filtering.
- Resource watcher enablement.
- In-memory dependency graph cache.
- Read-only REST API.
- Prometheus metrics.
- Grafana dashboard JSON.
- Prometheus alerting and recording rule examples.
- Helm chart.
- GitHub Actions CI, GitHub Pages documentation, and GHCR release publishing.

Planned work is tracked in ROADMAP.md.

## Design principles

1. No Secret values exported. KBeacon never emits Kubernetes Secret data or stringData through logs, metrics, or API responses.
2. Read-only Kubernetes access. KBeacon observes resources; it does not mutate Secrets or workloads.
3. Prometheus and Grafana first. KBeacon emits metrics and dashboards instead of running a custom UI or database.
4. Small operational footprint. One Agent Deployment per cluster; no queue, graph database, CRD, operator, or admission webhook required.
5. Bounded public contracts. Metrics, annotations, API responses, and Helm values are documented and reviewed carefully.

## Architecture

    Kubernetes API
        |
        | read-only watch/list
        v
    KBeacon Agent
        |
        | /metrics
        v
    Prometheus ---- optional remote_write ----> Grafana Mimir
        |
        | PromQL
        v
    Grafana dashboards and alerting

## Installation

Install the Helm chart:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.1.2

If the GHCR package is public, no Kubernetes image pull Secret is required.

If the GHCR package is private, create a pull Secret with a classic GitHub PAT that has read:packages:

    kubectl create namespace kbeacon-system --dry-run=client -o yaml | kubectl apply -f -

    read -rsp "GHCR read:packages token: " GHCR_TOKEN
    echo

    kubectl -n kbeacon-system create secret docker-registry ghcr-pull-secret \
      --docker-server=ghcr.io \
      --docker-username=<github-username> \
      --docker-password="${GHCR_TOKEN}" \
      --docker-email=<email> \
      --dry-run=client -o yaml | kubectl apply -f -

    unset GHCR_TOKEN

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.1.2 \
      --set 'imagePullSecrets[0].name=ghcr-pull-secret'

### Low-privilege install

If cluster policy does not allow the Agent to read Kubernetes Secret objects, disable Secret watching:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.1.2 \
      --set resourcesToWatch.core.secrets=false

KBeacon still discovers workload-to-Secret references, but referenced Secrets are reported as `exists=false` and dependency edges as `resolved=false` because Secret existence is unobservable.

### Metrics cardinality guard

`kbeacon_dependency_edges` is the most detailed metric family and includes workload and Secret names as labels.

For large clusters or shared Prometheus environments, disable detailed edge metrics:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.1.2 \
      --set metrics.edge.enabled=false

Aggregate metrics and the Agent API remain available.

## Verify

    kubectl -n kbeacon-system rollout status deploy/kbeacon
    kubectl -n kbeacon-system logs deploy/kbeacon --tail=100
    kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080

    curl -sS http://127.0.0.1:8081/readyz | jq
    curl -sS http://127.0.0.1:8081/api/v1/config | jq
    curl -sS http://127.0.0.1:8081/api/v1/secrets | jq
    curl -sS http://127.0.0.1:8081/api/v1/workloads | jq

## Local development

Minikube support is kept for local development and end-to-end smoke testing. It is not the production installation path.

    ./hack/local-dev/deploy-incluster-minikube.sh
    ./hack/local-dev/configure-prometheus-incluster.sh
    ./hack/local-dev/smoke-incluster.sh

The local workflow exercises Helm, RBAC, Service networking, Kubernetes informers, Prometheus scraping, and Grafana dashboards.

## Documentation

- Website: https://memoliyasti.github.io/kbeacon/
- Getting started: docs/getting-started.md
- Helm reference: docs/reference/helm.md
- Metrics reference: docs/reference/metrics.md
- Annotations reference: docs/reference/annotations.md
- API contract: docs/api/openapi.yaml
- Technical design: docs/technical-design.md

## Community

- Contributing: CONTRIBUTING.md
- Code of Conduct: CODE_OF_CONDUCT.md
- Security policy: SECURITY.md
- Support: SUPPORT.md
- Governance: GOVERNANCE.md
- Maintainers: MAINTAINERS.md
- Adopters: ADOPTERS.md

## Releases

Release tags use semantic versioning:

    git tag -a v0.1.2 -m "KBeacon v0.1.2"
    git push origin v0.1.2

The release workflow publishes:

- GitHub Release assets;
- Linux and macOS binaries;
- Helm chart package;
- SHA256 checksums;
- multi-arch container images for linux/amd64 and linux/arm64.

## License

Apache License 2.0. See LICENSE.

## Discovery modes

KBeacon supports four workload discovery modes:

| Mode | Behavior |
| --- | --- |
| `infer` | Discover Secret references from standard Pod spec fields. |
| `explicit` | Use only `kbeacon.io/watch-secrets` and `kbeacon.io/watch-secrets-json`. |
| `hybrid` | Combine inferred and explicit dependencies. This is the recommended default. |
| `disabled` | Ignore the workload. |

Discovery mode controls how dependencies are found. It is not the same thing as Kubernetes Secret `type`.

See the [Discovery modes guide](docs/user-guide/discovery-modes.md) and [Annotation reference](docs/reference/annotations.md).
