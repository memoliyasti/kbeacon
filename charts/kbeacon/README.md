# KBeacon Helm Chart

This chart deploys the KBeacon Agent as a read-only Kubernetes workload dependency intelligence component.

KBeacon observes workload metadata and Secret references, builds an in-memory dependency graph, exposes Prometheus metrics, and optionally renders Grafana dashboard ConfigMaps.

## Deployment model

The chart renders:

- one Agent `Deployment`;
- one internal `Service`;
- one `ServiceAccount`;
- RBAC for the enabled informer resources;
- optional Prometheus Operator `ServiceMonitor`;
- optional Grafana dashboard ConfigMap;
- optional ingress-only `NetworkPolicy`.

The chart does not install CRDs, operators, admission webhooks, databases, queues, or a custom UI.

## Required value

`cluster.name` is required. It is emitted as the logical cluster identity in metrics, API responses, and generated Agent configuration.

```yaml
cluster:
  name: prod-eu-1
```

## Image

The default image is published to GitHub Container Registry:

```yaml
image:
  repository: ghcr.io/memoliyasti/kbeacon
  tag: "0.2.3"
  pullPolicy: IfNotPresent
```

Use `image.digest` for immutable production deployments. When `image.digest` is set, the chart renders `repository@digest` instead of `repository:tag`.

## Discovery scope

KBeacon can run cluster-wide or namespace-scoped.

Cluster-wide mode is the default:

```yaml
rbac:
  create: true
  scope: cluster
```

Namespace-scoped mode renders `Role` and `RoleBinding` instead of cluster-wide RBAC:

```yaml
rbac:
  scope: namespace

discovery:
  namespaces:
    include:
      - payments
```

`discovery.namespaces.exclude` always takes precedence over `include`.

## Low-privilege mode

KBeacon can operate without reading Kubernetes Secret objects.

```yaml
resourcesToWatch:
  core:
    secrets: false
```

In this mode, the Agent still discovers workload-to-Secret references from workload specs and annotations. Referenced Secrets are represented as unobservable, with `exists=false` and `resolved=false`.

## Resource watcher control

Each implemented informer can be enabled or disabled through values:

```yaml
resourcesToWatch:
  core:
    secrets: true
    pods: true
  apps:
    deployments: true
    statefulSets: true
    daemonSets: true
  batch:
    jobs: true
    cronJobs: true
```

Disabled resources are reported as optional in `/readyz` and are not started as informers.

## Metrics

KBeacon exposes metrics on the Agent HTTP port at `/metrics`.

Detailed dependency edge metrics can be disabled when cardinality is a concern:

```yaml
metrics:
  edge:
    enabled: false
```

Aggregate graph metrics and the read-only Agent API remain available when detailed edge metrics are disabled.

## Prometheus integration

Prometheus Operator users can enable a ServiceMonitor:

```yaml
serviceMonitor:
  enabled: true
  labels:
    release: kube-prometheus-stack
  honorLabels: true
```

Clusters that use annotation-based scraping can enable Service annotations instead:

```yaml
prometheus:
  scrapeAnnotations:
    enabled: true
    target: service
```

## Grafana dashboards

Dashboard ConfigMaps are disabled by default:

```yaml
dashboards:
  enabled: false
```

When enabled, the chart renders dashboard JSON from `charts/kbeacon/dashboards/`.

Included dashboards:

- `KBeacon / Cluster Overview`
- `KBeacon / Secret Dependency Map`
- `KBeacon / Team Overview`
- `KBeacon / Dependency Graph Explorer`

The Dependency Graph Explorer requires `metrics.edge.enabled=true` because it is powered by `kbeacon_dependency_edges`.

## Security defaults

The chart defaults to a non-root, read-only container security posture:

```yaml
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65532
  runAsGroup: 65532
  fsGroup: 65532

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

KBeacon never exports Kubernetes Secret values. Secret names and dependency metadata can still be sensitive and should be protected in Prometheus, Grafana, logs, and Agent API access.

## Values reference

The full values contract is documented inline in `values.yaml` and in `docs/reference/helm.md`.
