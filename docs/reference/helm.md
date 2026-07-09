# KBeacon Helm reference

KBeacon deploys as one lightweight Deployment, one Service, read-only RBAC, and optional dashboard and Prometheus resources.

The chart does not install CRDs, operators, admission webhooks, databases, queues, or a custom UI.

## Public repository install

```bash
helm repo add kbeacon-release https://memoliyasti.github.io/kbeacon/charts
helm repo update kbeacon-release

helm search repo kbeacon-release/kbeacon --versions | head

VERSION="$(helm search repo kbeacon-release/kbeacon --versions | awk 'NR==2 {print $2}')"

helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --wait \
  --timeout 10m
```

## Required value

```yaml
cluster:
  name: prod-eu-1
```

`cluster.name` is required and is used as the cluster label in metrics and API responses.

## Image values

```yaml
image:
  repository: ghcr.io/memoliyasti/kbeacon
  tag: "<release version>"
  digest: ""
  pullPolicy: IfNotPresent
```

When `image.digest` is set, the chart renders `repository@digest` instead of `repository:tag`.

## Discovery values

```yaml
discovery:
  defaultMode: hybrid
  includeImagePullSecrets: true
  includeInitContainers: true
  includeEphemeralContainers: true
  readPodTemplateAnnotations: true
  namespaces:
    include: []
    exclude:
      - kube-system
      - kube-public
      - kube-node-lease
  resyncInterval: 10h
  reconcile:
    debounce: 250ms
```

Supported discovery modes:

| Mode | Behavior |
| --- | --- |
| `infer` | Discover Secret references from workload specs. |
| `explicit` | Use only KBeacon explicit dependency annotations. |
| `hybrid` | Combine inferred and explicit dependencies. |
| `disabled` | Ignore the workload. |

Namespace behavior:

- `include: []` means all namespaces are eligible unless excluded.
- Non-empty `include` acts as an allow-list.
- `exclude` overrides `include`.

## Watched resources

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
    replicaSets: true
  batch:
    jobs: true
    cronJobs: true
  networking:
    ingresses: true
  certManager:
    certificates: false
  externalSecrets:
    externalSecrets: false
  secretsStore:
    secretProviderClasses: false
  strimzi:
    kafkaConnectors: false
  confluent:
    connectors: false
```

Optional CRD watchers should be enabled only when the matching CRDs are installed in the cluster.

## Low-privilege mode

```bash
helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set resourcesToWatch.core.secrets=false \
  --wait \
  --timeout 10m
```

In this mode:

- the chart does not render Secret RBAC rules;
- the Agent does not start the Secret informer;
- workload-to-Secret edges are still discovered from workload specs and annotations;
- referenced Secrets are represented with `exists=false`;
- dependency edges have `resolved=false`.

## Namespace-scoped mode

```bash
helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace payments \
  --set cluster.name=prod-eu-1 \
  --set rbac.scope=namespace \
  --set-string discovery.namespaces.include[0]=payments \
  --wait \
  --timeout 10m
```

`rbac.scope` must be either `cluster` or `namespace`.

## Prometheus integration

Prometheus Operator `ServiceMonitor`:

```yaml
serviceMonitor:
  enabled: false
  interval: 30s
  scrapeTimeout: 10s
  honorLabels: true
```

Scrape annotations:

```yaml
prometheus:
  scrapeAnnotations:
    enabled: false
    target: service
    path: /metrics
    port: "8080"
```

## Grafana dashboards

```yaml
dashboards:
  enabled: false
  labels:
    grafana_dashboard: "1"
```

When enabled, the chart renders dashboard ConfigMaps from `charts/kbeacon/dashboards/`.

## Metrics

```yaml
metrics:
  edge:
    enabled: true
  runtime:
    enabled: true
```

`metrics.edge.enabled=false` disables the high-cardinality `kbeacon_dependency_edges` metric family. Aggregate metrics and API responses remain available.

## Security defaults

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

## Supported resource watcher values

These value paths are part of the public Helm configuration contract and are kept here so install docs, Helm values, and supported-resource validation stay aligned.

| Resource family | Helm value | Dependency sources | Notes |
| --- | --- | --- | --- |
| Pods | `resourcesToWatch.core.pods` | `env.secretKeyRef`, `envFrom.secretRef`, `volumes.secret`, `imagePullSecrets` | Core workload reference discovery. |
| Secrets | `resourcesToWatch.core.secrets` | Secret metadata observation | Disable for low-privilege reference-only mode. |
| Deployments | `resourcesToWatch.apps.deployments` | Pod template Secret references | Enabled by default. |
| StatefulSets | `resourcesToWatch.apps.statefulSets` | Pod template Secret references | Enabled by default. |
| DaemonSets | `resourcesToWatch.apps.daemonSets` | Pod template Secret references | Enabled by default. |
| ReplicaSets | `resourcesToWatch.apps.replicaSets` | Pod template Secret references | Used for owner-resolution coverage. |
| Jobs | `resourcesToWatch.batch.jobs` | Pod template Secret references | Enabled by default. |
| CronJobs | `resourcesToWatch.batch.cronJobs` | Pod template Secret references | Enabled by default. |
| Ingress | `resourcesToWatch.networking.ingresses` | `ingress.tls` | Discovers TLS Secret dependencies. |
| cert-manager Certificate | `resourcesToWatch.certManager.certificates` | `cert-manager.certificate.spec.secretName` | Enable only when cert-manager CRDs are installed. |
| ExternalSecret | `resourcesToWatch.externalSecrets.externalSecrets` | `externalsecret.spec.target.name` | Enable only when External Secrets Operator CRDs are installed. |
| SecretProviderClass | `resourcesToWatch.secretsStore.secretProviderClasses` | `secretproviderclass.spec.secretObjects.secretName` | Enable only when Secrets Store CSI Driver CRDs are installed. |
| Strimzi KafkaConnector | `resourcesToWatch.strimzi.kafkaConnectors` | `strimzi.configProvider.secret` | Enable only when Strimzi CRDs are installed. |
| Confluent Connector | `resourcesToWatch.confluent.connectors` | `confluent.connector.mountedSecret` | Enable only when Confluent for Kubernetes CRDs are installed. |

Optional CRD watchers are disabled unless explicitly enabled. The chart only renders the matching read-only RBAC rules when the corresponding watcher value is enabled.
## Supported resources

For the full supported watcher matrix and dependency source paths, see [Supported resources](supported-resources.md).


## Validation

```bash
helm lint ./charts/kbeacon --set cluster.name=ci

helm template kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --set cluster.name=ci \
  --set dashboards.enabled=true
```
