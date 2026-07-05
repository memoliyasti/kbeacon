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
  tag: "0.3.9"
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
    serviceAccounts: true
    pods: true
  apps:
    deployments: true
    statefulSets: true
    daemonSets: true
  networking:
    ingresses: true
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

## Values schema

The chart includes `values.schema.json` for Helm values validation.

The schema validates required cluster identity, enum-style options such as `rbac.scope`, `discovery.defaultMode`, and `log.level`, and the structure of common chart configuration blocks.

Run schema validation with:

```bash
make helm-schema-lint
```

## ServiceAccount imagePullSecrets fallback

When `discovery.includeImagePullSecrets=true`, KBeacon discovers direct Pod-level `spec.imagePullSecrets` references.

If a workload does not set Pod-level `imagePullSecrets`, KBeacon can fall back to the workload ServiceAccount and discover Secrets listed in `serviceAccount.imagePullSecrets`.

This requires the ServiceAccount watcher and RBAC rule:

```yaml
resourcesToWatch:
  core:
    serviceAccounts: true
```

Disable this watcher only when ServiceAccount metadata should not be observed:

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --set cluster.name=prod-eu-1 \
  --set resourcesToWatch.core.serviceAccounts=false
```

## Ingress TLS Secret discovery

KBeacon can watch Kubernetes Ingress resources and discover TLS Secrets from `spec.tls[].secretName`.

```yaml
resourcesToWatch:
  networking:
    ingresses: true
```

Disable Ingress watching and Ingress RBAC when it is not needed:

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --set cluster.name=prod-eu-1 \
  --set resourcesToWatch.networking.ingresses=false
```

## Service exposure and NetworkPolicy

The chart defaults to an internal `ClusterIP` Service. Keep `service.type=ClusterIP` for normal installs and use `kubectl port-forward`, an internal platform proxy, or an internal-only ingress path for controlled access.

`NodePort` and `LoadBalancer` are schema-valid Kubernetes Service types, but they expose the read-only Agent API more broadly. Use them only with explicit network controls.

When your cluster has a NetworkPolicy controller, enable `networkPolicy.enabled=true` and set `networkPolicy.ingress.from` to the Prometheus, Grafana, or platform namespaces and Pods that are allowed to reach the Agent.

Example values:

    service:
      type: ClusterIP

    networkPolicy:
      enabled: true
      ingress:
        from:
          - podSelector:
              matchLabels:
                app: prometheus

## Replica count

KBeacon v0.3.x is intentionally single-replica. Keep `replicaCount=1`.

Each Agent replica builds its own in-memory dependency graph. Running more than one replica without leader election can duplicate Prometheus metrics and expose replica-local API snapshots. The chart therefore rejects `replicaCount` values other than `1`.

High availability with leader election is tracked as future work.

## Projected Secret volumes

Kubernetes projected volumes can include Secret projections. KBeacon discovers these references from Pod specs and workload Pod templates.

Supported source path:

    spec.volumes[].projected.sources[].secret.name

KBeacon records these dependencies with source type `volumes.projected.sources.secret`. The dependency is namespace-local to the workload, matching Kubernetes Secret volume semantics.

## Privacy and redaction

Redact Secret key names in Agent API dependency source paths:

    privacy:
      redaction:
        secretKeys: true

Secret names and namespaces remain visible because they are part of the dependency graph.

## Kind E2E smoke test

KBeacon includes a Kind-based end-to-end smoke test for the chart, RBAC, Kubernetes informers, projected Secret volume discovery, privacy redaction, and the read-only Agent API.

Run it locally when docker, kind, kubectl, helm, and python3 are available:

    make kind-e2e-smoke

The test builds a local kbeacon-agent:e2e image, loads it into a temporary Kind cluster, installs the Helm chart, creates a small workload graph, and verifies the Agent API.

## Supported resource matrix

The implemented Kubernetes resource and dependency source matrix is documented in `docs/reference/supported-resources.md`.

Use that page as the source of truth for what KBeacon watches today and what is only future roadmap scope.

## cert-manager Certificate discovery

Enable this optional watcher only when cert-manager CRDs are installed:

```yaml
resourcesToWatch:
  certManager:
    certificates: true
```

KBeacon models `Certificate.spec.secretName` as a dependency edge to the target Kubernetes Secret. The chart adds read-only RBAC for `cert-manager.io` `certificates` only when the watcher is enabled.

## ExternalSecret discovery

Enable this optional watcher only when External Secrets Operator CRDs are installed:

~~~yaml
resourcesToWatch:
  externalSecrets:
    externalSecrets: true
~~~

KBeacon models each `ExternalSecret` target Kubernetes Secret as a dependency edge. It uses `spec.target.name` first and falls back to the `ExternalSecret` object name when `spec.target.name` is omitted.

The chart adds read-only RBAC for `external-secrets.io` `externalsecrets` only when the watcher is enabled.

## SecretProviderClass discovery

Enable this optional watcher only when Secrets Store CSI Driver CRDs are installed:

~~~yaml
resourcesToWatch:
  secretsStore:
    secretProviderClasses: true
~~~

KBeacon models each `spec.secretObjects[*].secretName` synced Kubernetes Secret as a dependency edge from the `SecretProviderClass`.

The chart adds read-only RBAC for `secrets-store.csi.x-k8s.io` `secretproviderclasses` only when the watcher is enabled.

KBeacon does not read external provider values, mounted file contents, or Kubernetes Secret data.

## Kafka connector discovery

Enable these optional watchers only when the matching CRDs are installed.

For Strimzi KafkaConnector discovery:

~~~yaml
resourcesToWatch:
  strimzi:
    kafkaConnectors: true
~~~

KBeacon parses Strimzi Kubernetes Config Provider Secret references in `spec.config` string values and models the referenced Kubernetes Secrets as dependency edges from the `KafkaConnector`.

For Confluent for Kubernetes Connector discovery:

~~~yaml
resourcesToWatch:
  confluent:
    connectors: true
~~~

KBeacon models `spec.connectRest.authentication.*.secretRef` and `${file:/mnt/secrets/<secret>/...:key}` mounted Secret file references as dependency edges from the `Connector`.

The chart adds read-only RBAC for `kafka.strimzi.io` `kafkaconnectors` and `platform.confluent.io` `connectors` only when the matching watcher is enabled.

KBeacon does not call Kafka Connect REST APIs, read connector plugin payloads, inspect mounted file contents, or read Kubernetes Secret data.

`resourcesToWatch.apps.replicaSets=true` is enabled by default as an owner-resolution cache. ReplicaSets are watched read-only so KBeacon can map ReplicaSet-owned Pods back to Deployments and avoid duplicate Pod workload nodes. ReplicaSets are not emitted as primary workload nodes.
