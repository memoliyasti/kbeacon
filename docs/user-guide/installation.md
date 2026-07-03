# Installation

KBeacon is packaged as a Helm chart and runs as one read-only Agent Deployment per Kubernetes cluster.

The recommended production pattern is to manage deployment through version-controlled Helm values rather than long command-line overrides.

## Deployment prerequisites

KBeacon requires:

- a Kubernetes cluster;
- Helm-compatible deployment tooling;
- read-only RBAC for the Kubernetes resources selected in `resourcesToWatch`;
- a Prometheus scrape path if metrics are required;
- optional Grafana dashboard discovery when dashboard ConfigMaps are enabled.

## Release image

The default image is published to GitHub Container Registry:

```yaml
image:
  repository: ghcr.io/memoliyasti/kbeacon
  tag: "0.3.2"
  pullPolicy: IfNotPresent
```

The project GHCR package is intended to be public. The default deployment path does not require an image pull Secret.

For production, prefer digest pinning when your deployment process supports it:

```yaml
image:
  repository: ghcr.io/memoliyasti/kbeacon
  digest: sha256:<digest>
```

When `image.digest` is set, the chart renders `repository@digest` and ignores `image.tag`.

## Required cluster identity

`cluster.name` is required and should be stable for the lifetime of the logical cluster.

```yaml
cluster:
  name: prod-eu-1
  environment: prod
  region: eu
```

The cluster name is emitted in metrics, API responses, generated Agent configuration, and dashboard variables.

## Standard deployment profile

The default profile runs cluster-wide discovery with all implemented Kubernetes resource watchers enabled.

```yaml
rbac:
  create: true
  scope: cluster

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

Use this profile when the Agent is allowed to observe Secret metadata and workload specifications across the cluster.

## Namespace-scoped profile

Namespace-scoped deployment renders namespace-local RBAC and should be paired with an explicit namespace allow-list.

```yaml
rbac:
  scope: namespace

discovery:
  namespaces:
    include:
      - payments
```

Use this profile when each namespace or tenant has an independently managed Agent deployment.

## Low-privilege profile

KBeacon can run without reading Kubernetes Secret objects.

```yaml
resourcesToWatch:
  core:
    secrets: false
```

In this profile:

- workload-to-Secret references are still discovered from workload specs and annotations;
- referenced Secrets are represented as unobservable;
- API responses and metrics use `exists=false` and `resolved=false` for unobserved Secret objects;
- Secret type, Secret annotation metadata, and Secret change counters are unavailable.

Use this profile when cluster policy does not allow observability components to read Secret objects.

## Prometheus integration

KBeacon exposes metrics on the Agent HTTP port at `/metrics`.

Prometheus Operator profile:

```yaml
serviceMonitor:
  enabled: true
  labels:
    release: kube-prometheus-stack
  honorLabels: true
```

Annotation-based scrape profile:

```yaml
prometheus:
  scrapeAnnotations:
    enabled: true
    target: service
    path: /metrics
    port: "8080"
```

Centralized Prometheus configurations can scrape the chart-rendered Service directly.

## Grafana dashboards

Dashboard ConfigMaps are optional.

```yaml
dashboards:
  enabled: true
  labels:
    grafana_dashboard: "1"
```

Enable this only when your Grafana deployment discovers dashboard ConfigMaps by label.

Included dashboards:

- `KBeacon / Cluster Overview`
- `KBeacon / Secret Dependency Map`
- `KBeacon / Team Overview`
- `KBeacon / Dependency Graph Explorer`

The Dependency Graph Explorer requires `metrics.edge.enabled=true`.

## Metrics cardinality profile

The detailed edge metric powers edge-level troubleshooting and graph dashboards.

```yaml
metrics:
  edge:
    enabled: true
```

For very large clusters or strict cardinality budgets, disable detailed edge metrics:

```yaml
metrics:
  edge:
    enabled: false
```

Aggregate metrics and the read-only Agent API remain available.

## Security posture

The chart defaults to a non-root, read-only container security posture.

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

KBeacon uses read-only Kubernetes permissions and does not export Kubernetes Secret values. Secret names and dependency metadata may still be sensitive and should be protected in Prometheus, Grafana, logs, and Agent API access.

## Validation

Validate rendered manifests before promoting values into production.

Recommended validation coverage:

- chart linting;
- default manifest rendering;
- low-privilege manifest rendering;
- namespace-scoped RBAC rendering;
- dashboard JSON validation when dashboards are enabled;
- Prometheus rule validation when alerting rules are used.

The repository validation target covers these checks through `make validate-ci`.

## Next steps

- Configuration reference: `docs/user-guide/configuration.md`
- Helm values reference: `docs/reference/helm.md`
- Metrics reference: `docs/reference/metrics.md`
- Dashboard guide: `docs/user-guide/dashboards.md`
- Security guide: `docs/operator-guide/security.md`

## Internal Agent API access

The default Service type is `ClusterIP`. This keeps the Agent API internal to the cluster.

Use port-forwarding for ad hoc local access:

    kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080

Avoid exposing the Agent API through `NodePort` or `LoadBalancer` unless your environment applies authentication, authorization, and network restrictions outside KBeacon.

## Replica count and availability

KBeacon v0.3.x runs as one Agent replica per cluster.

Keep `replicaCount=1` for normal installs. Multi-replica operation is not supported until leader election is implemented, because each Agent replica independently watches Kubernetes and builds its own in-memory graph.

## Kind E2E smoke test

KBeacon includes a Kind-based end-to-end smoke test for the chart, RBAC, Kubernetes informers, projected Secret volume discovery, privacy redaction, and the read-only Agent API.

Run it locally when docker, kind, kubectl, helm, and python3 are available:

    make kind-e2e-smoke

The test builds a local kbeacon-agent:e2e image, loads it into a temporary Kind cluster, installs the Helm chart, creates a small workload graph, and verifies the Agent API.
