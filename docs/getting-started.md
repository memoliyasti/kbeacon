# Getting started

KBeacon helps platform, SRE, DevOps, and security engineering teams understand Kubernetes Secret blast radius.

It builds a current workload-to-Secret dependency graph from Kubernetes workload metadata, exposes Prometheus metrics, provides a read-only Agent API, and ships Grafana dashboards.

## Start with the operating model

KBeacon is intentionally deployed as one lightweight Agent per Kubernetes cluster.

The Agent:

- watches selected Kubernetes resources with read-only permissions;
- extracts workload-to-Secret references;
- keeps the current dependency graph in memory;
- exposes metrics for Prometheus or Grafana Mimir;
- exposes a read-only API for inspection and automation;
- renders optional Grafana dashboard ConfigMaps through Helm.

KBeacon does not export Kubernetes Secret values.

## Choose a deployment profile

Most teams should start by choosing one of these profiles.

| Profile | Use when | Key values |
| --- | --- | --- |
| Standard cluster-wide | Platform team owns cluster-level observability and can read Secret metadata. | `rbac.scope=cluster`, `resourcesToWatch.core.secrets=true` |
| Low privilege | Security policy does not allow the Agent to read Secret objects. | `resourcesToWatch.core.secrets=false` |
| Namespace scoped | Tenants or teams run KBeacon only for selected namespaces. | `rbac.scope=namespace`, `discovery.namespaces.include` |
| Dashboard enabled | Grafana discovers dashboard ConfigMaps by label. | `dashboards.enabled=true` |
| Cardinality constrained | Prometheus cardinality budget is strict. | `metrics.edge.enabled=false` |

The full installation profile guide is in `docs/user-guide/installation.md`.

## Minimal values

A production values file should start with a stable cluster identity.

```yaml
cluster:
  name: prod-eu-1
  environment: prod
  region: eu
```

The cluster name is emitted in Prometheus metrics, Agent API responses, generated Agent configuration, and Grafana dashboard variables.

## Image source

The default release image is published to GitHub Container Registry.

```yaml
image:
  repository: ghcr.io/memoliyasti/kbeacon
  tag: "0.3.11"
  pullPolicy: IfNotPresent
```

The project GHCR package is intended to be public. For production environments that require immutable artifacts, use `image.digest`.

## Discovery basics

KBeacon supports four workload discovery modes.

| Mode | Behavior |
| --- | --- |
| `infer` | Discover Secret references from Kubernetes workload specs. |
| `explicit` | Use only KBeacon dependency annotations. |
| `hybrid` | Combine inferred and explicit dependencies. |
| `disabled` | Ignore the workload. |

The default mode is `hybrid`.

KBeacon currently extracts dependencies from:

- `env.valueFrom.secretKeyRef`;
- `envFrom.secretRef`;
- `volumes.secret`;
- `imagePullSecrets`;
- `kbeacon.io/watch-secrets`;
- `kbeacon.io/watch-secrets-json`.

Discovery behavior is documented in `docs/user-guide/discovery-modes.md` and `docs/reference/annotations.md`.

## Observability path

KBeacon exposes metrics at `/metrics` on the Agent HTTP port.

Prometheus integration can be configured through:

- Prometheus Operator `ServiceMonitor`;
- Service scrape annotations;
- centrally managed scrape configuration.

Grafana dashboards are optional and are rendered through Helm when `dashboards.enabled=true`.

Dashboard documentation is available in `docs/user-guide/dashboards.md`.

## Security baseline

KBeacon is designed around a conservative operating model:

- read-only Kubernetes permissions;
- no mutation of Secrets or workloads;
- no Secret value export in metrics, logs, or API responses;
- optional low-privilege mode without Secret object reads;
- internal or controlled access for the Agent API.

Secret names and dependency metadata may still be sensitive. Protect Prometheus, Grafana, logs, and Agent API access accordingly.

## Recommended reading order

1. `docs/user-guide/installation.md`
2. `docs/user-guide/configuration.md`
3. `docs/user-guide/discovery-modes.md`
4. `docs/reference/annotations.md`
5. `docs/reference/metrics.md`
6. `docs/user-guide/dashboards.md`
7. `docs/api/openapi.yaml`

## Validation

Before promoting a values profile, validate chart rendering, RBAC mode, dashboard JSON, Prometheus rules, and release metadata.

The repository validation entry point is:

```bash
make validate-ci
```
