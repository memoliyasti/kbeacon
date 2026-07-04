# KBeacon

[![CI](https://github.com/memoliyasti/kbeacon/actions/workflows/ci.yaml/badge.svg)](https://github.com/memoliyasti/kbeacon/actions/workflows/ci.yaml)
[![Release](https://github.com/memoliyasti/kbeacon/actions/workflows/release.yaml/badge.svg)](https://github.com/memoliyasti/kbeacon/actions/workflows/release.yaml)
[![Docs](https://github.com/memoliyasti/kbeacon/actions/workflows/pages.yaml/badge.svg)](https://github.com/memoliyasti/kbeacon/actions/workflows/pages.yaml)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kbeacon)](https://artifacthub.io/packages/search?repo=kbeacon)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

KBeacon is a Kubernetes Secret dependency intelligence agent for platform, SRE, DevOps, and security engineering teams.

It answers one operational question:

> If this Secret changes, which workloads are affected?

KBeacon watches Kubernetes resources with read-only access, builds an in-memory workload-to-Secret dependency graph, exports Prometheus metrics, exposes a read-only Agent API, and ships Grafana dashboards for blast-radius analysis.

KBeacon never reads, logs, exports, or stores Kubernetes Secret values. It only uses Secret names, namespaces, metadata, and workload references.

## Why KBeacon

Secret rotations, certificate renewals, registry credential changes, and database credential updates can affect many workloads. In many clusters, dependency information exists indirectly across manifests, Pod specs, annotations, and team ownership conventions.

KBeacon turns that scattered metadata into a current dependency graph that helps teams understand:

- which workloads reference a Secret;
- which Secrets have broad fan-out;
- which teams and namespaces are affected;
- which references are unresolved or unobservable;
- which Secret changes need extra review before rollout.

## Core capabilities

- Kubernetes-native discovery using `client-go` shared informers.
- Workload coverage for Pods, Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, and Ingress TLS references.
- Dependency extraction from standard Pod spec fields: `env.valueFrom.secretKeyRef`, `envFrom.secretRef`, `volumes.secret`, `volumes.projected.sources.secret`, and `imagePullSecrets`.
- ServiceAccount imagePullSecrets fallback discovery when workloads omit Pod-level `imagePullSecrets`.
- Ingress TLS Secret discovery from networking.k8s.io/v1 Ingress spec.tls[].secretName.
- Explicit dependency modeling through KBeacon annotations.
- Discovery modes: `infer`, `explicit`, `hybrid`, and `disabled`.
- Workload ownership and classification from KBeacon annotations or existing Kubernetes labels.
- Namespace include and exclude filtering.
- Low-privilege mode without Kubernetes Secret object reads.
- Read-only Agent API with filtering and bounded pagination.
- Prometheus metrics with a configurable edge-cardinality guard.
- Grafana dashboards, including a dependency Node Graph explorer.
- Helm chart, release artifacts, public GHCR images, and CI validation.

## Architecture

```text
Kubernetes API
    |
    | read-only watch/list
    v
KBeacon Agent
    |
    | /metrics and read-only API
    v
Prometheus or Grafana Mimir
    |
    | PromQL
    v
Grafana dashboards and alerting
```

The Agent keeps the current dependency graph in memory. Prometheus or Mimir store metric history. Grafana provides the operational view.

## Deployment model

KBeacon is packaged as a Helm chart and runs as one lightweight Agent Deployment per Kubernetes cluster.

The chart can be configured for:

- cluster-wide or namespace-scoped RBAC;
- full Secret metadata observation or low-privilege reference-only discovery;
- Prometheus Operator `ServiceMonitor`, annotation-based scraping, or static scrape configuration;
- optional Grafana dashboard ConfigMaps;
- detailed edge metrics enabled or disabled according to cardinality requirements.

The default release image is published to GitHub Container Registry under:

```text
ghcr.io/memoliyasti/kbeacon
```

The project GHCR package is intended to be public, so the default deployment path does not require an image pull Secret.

## Discovery model

KBeacon supports four workload discovery modes.

| Mode | Behavior |
| --- | --- |
| `infer` | Discover Secret references from Kubernetes workload specs. |
| `explicit` | Use only KBeacon dependency annotations. |
| `hybrid` | Combine inferred and explicit dependencies. |
| `disabled` | Ignore the workload. |

Explicit Secret references use this grammar:

```text
secret
secret#key
namespace/secret
namespace/secret#key
```

## Security posture

- KBeacon does not expose Kubernetes Secret values.
- Kubernetes permissions are read-only.
- Secret object watching can be disabled for low-privilege environments.
- Secret names and dependency metadata may still be sensitive and should be protected in Prometheus, Grafana, logs, and API access.
- Secret key names in API source paths can be redacted with `privacy.redaction.secretKeys=true`.
- The Agent API is intended for internal cluster or controlled platform access.

## Observability

KBeacon exports Prometheus metrics for:

- dependency edge count;
- observed Secret and workload counts;
- Secret impact score;
- affected workload, team, and namespace counts;
- unresolved Secret references;
- Kubernetes informer health;
- graph rebuild latency and watch event activity.

Dashboard JSON files are maintained in:

```text
dashboards/
charts/kbeacon/dashboards/
```

The dashboard set includes cluster overview, team overview, Secret dependency map, and dependency graph exploration.

## Documentation

- Website: https://memoliyasti.github.io/kbeacon/
- Getting started: `docs/getting-started.md`
- Installation: `docs/user-guide/installation.md`
- CLI: `docs/user-guide/cli.md`
- Alert runbooks: `docs/operator-guide/runbooks.md`
- Human-readable Secret impact reports: `kbeaconctl impact report <namespace> <secret>`
- Portable API snapshots: `kbeaconctl snapshot export --output kbeacon-snapshot.json`.
- Snapshot diffs: `kbeaconctl snapshot diff old.json new.json`.
- Snapshot diff markdown for PR comments: `kbeaconctl snapshot diff --format markdown old.json new.json`.
- Release assets include `kbeaconctl` binaries for Linux and macOS.
- Configuration: `docs/user-guide/configuration.md`
- Discovery modes: `docs/user-guide/discovery-modes.md`
- Dashboards: `docs/user-guide/dashboards.md`
- Helm reference: `docs/reference/helm.md`
- Supported resources: docs/reference/supported-resources.md
- Metrics reference: `docs/reference/metrics.md`
- Annotation reference: `docs/reference/annotations.md`
- Agent API: `docs/api/openapi.yaml`
- Technical design: `docs/technical-design.md`

## Development

Primary validation entry point:

```bash
make validate-ci
```

This runs Go formatting, tests, build, Helm rendering, Prometheus rule validation, dashboard validation, demo linting, scale fixture checks, stale checks, and release metadata checks.

## Releases

Releases use semantic version tags and publish GitHub Release assets, Linux and macOS binaries, a Helm chart package, SHA256 checksums, and multi-arch container images for `linux/amd64` and `linux/arm64`.

Current release line:

```text
v0.3.3
```

## Community

- Contributing: `CONTRIBUTING.md`
- Code of Conduct: `CODE_OF_CONDUCT.md`
- Security policy: `SECURITY.md`
- Support: `SUPPORT.md`
- Governance: `GOVERNANCE.md`
- Maintainers: `MAINTAINERS.md`
- Adopters: `ADOPTERS.md`

## License

MIT License. See `LICENSE` and `NOTICE`.

## Kind E2E smoke test

KBeacon includes a Kind-based end-to-end smoke test for the chart, RBAC, Kubernetes informers, projected Secret volume discovery, privacy redaction, and the read-only Agent API.

Run it locally when docker, kind, kubectl, helm, and python3 are available:

    make kind-e2e-smoke

The test builds a local kbeacon-agent:e2e image, loads it into a temporary Kind cluster, installs the Helm chart, creates a small workload graph, and verifies the Agent API.

## Supply chain security

Releases publish checksums, SPDX JSON SBOM files, signed Helm chart provenance, and release artifact attestations. Release container images are built with provenance and SBOM metadata enabled.
