# Helm Reference

KBeacon is deployed through the `charts/kbeacon` Helm chart as one read-only Agent Deployment per Kubernetes cluster.

This page documents the implemented chart values and the operational profiles they enable.

## Chart scope

The chart renders Kubernetes primitives only:

- `Deployment`
- `Service`
- `ServiceAccount`
- `Role` / `RoleBinding` or `ClusterRole` / `ClusterRoleBinding`
- optional `ServiceMonitor`
- optional Grafana dashboard `ConfigMap`
- optional `NetworkPolicy`

The chart does not install KBeacon CRDs, an operator, admission webhooks, databases, queues, or a custom UI.

## Required value

`cluster.name` is required for normal deployments.

```yaml
cluster:
  name: prod-eu-1
```

The value is used as the logical cluster identity in:

- Prometheus metric labels;
- Agent API responses;
- generated Agent configuration;
- dashboard variables.

## Image configuration

Default image values:

```yaml
image:
  repository: ghcr.io/memoliyasti/kbeacon
  tag: "0.3.17"
  digest: ""
  pullPolicy: IfNotPresent
```

Use `image.digest` for immutable production deployments. When a digest is set, the chart renders `repository@digest` and ignores `image.tag`.

`imagePullSecrets` is available for private or mirrored registries:

```yaml
imagePullSecrets:
  - name: registry-pull-secret
```

The default project GHCR package is intended to be public and does not require an image pull Secret.

## Cluster and runtime metadata

```yaml
cluster:
  name: prod-eu-1
  environment: prod
  region: eu

log:
  level: info
```

`cluster.environment` and `cluster.region` are reserved metadata fields in the generated Agent configuration.

## Agent runtime

```yaml
agent:
  http:
    port: 8080
  shutdownGracePeriod: 15s

health:
  livenessPath: /healthz
  readinessPath: /readyz
```

The Agent exposes health, metrics, and read-only API endpoints on the configured HTTP port.

## Discovery configuration

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
| `infer` | Discover Secret references from workload Pod specs. |
| `explicit` | Use only KBeacon explicit dependency annotations. |
| `hybrid` | Combine inferred and explicit dependencies. |
| `disabled` | Ignore matching workloads. |

Namespace behavior:

- `include: []` means all namespaces are eligible unless excluded.
- non-empty `include` acts as an allow-list.
- `exclude` takes precedence over `include`.

## Metadata label fallback

KBeacon can map existing Kubernetes workload labels into ownership and classification fields.

```yaml
discovery:
  metadataLabels:
    enabled: true
    ownerTeam:
      - app.kubernetes.io/team
      - team
      - owner-team
      - ownerTeam
      - technical-owner
      - technicalOwner
      - business-owner
      - businessOwner
    service:
      - app.kubernetes.io/name
      - app
      - service
      - service-name
      - serviceName
    environment:
      - app.kubernetes.io/environment
      - environment
      - env
      - stage
    criticality:
      - app.kubernetes.io/criticality
      - criticality
      - priority
      - tier
      - slo-tier
```

KBeacon annotations have higher precedence than label fallback.

## Resource watchers

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

Implemented watcher values:

| Value path | Resource |
| --- | --- |
| `resourcesToWatch.core.secrets` | `Secret` |
| `resourcesToWatch.core.serviceAccounts` | `ServiceAccount` |
| `resourcesToWatch.core.pods` | `Pod` |
| `resourcesToWatch.apps.deployments` | `Deployment` |
| `resourcesToWatch.apps.replicaSets` | `ReplicaSet` owner-resolution cache |
| `resourcesToWatch.apps.statefulSets` | `StatefulSet` |
| `resourcesToWatch.apps.daemonSets` | `DaemonSet` |
| `resourcesToWatch.batch.jobs` | `Job` |
| `resourcesToWatch.batch.cronJobs` | `CronJob` |
| `resourcesToWatch.networking.ingresses` | `Ingress` |
| `resourcesToWatch.certManager.certificates` | `Certificate` |
| `resourcesToWatch.externalSecrets.externalSecrets` | `ExternalSecret` |
| `resourcesToWatch.secretsStore.secretProviderClasses` | `SecretProviderClass` |
| `resourcesToWatch.strimzi.kafkaConnectors` | `KafkaConnector` |
| `resourcesToWatch.confluent.connectors` | `Connector` |

Disabled resources are not started as informers. They are represented as optional in readiness status.

## Low-privilege profile

Low-privilege mode disables Secret object watching:

```yaml
resourcesToWatch:
  core:
    secrets: false
```

In this profile:

- the chart does not render Secret RBAC rules;
- the Agent does not start the Secret informer;
- workload-to-Secret references are still discovered from workload specs and annotations;
- referenced Secrets are represented with `exists=false`;
- dependency edges are marked `resolved=false`;
- Secret type and change metadata are unavailable.

## Metrics

```yaml
metrics:
  edge:
    enabled: true
  runtime:
    enabled: true
```

`metrics.edge.enabled=false` disables the high-cardinality `kbeacon_dependency_edges` metric family. Aggregate graph metrics and the Agent API remain available.

`metrics.runtime.enabled=false` disables runtime collector and recorder metrics. Graph metrics remain available.

## Service

```yaml
service:
  type: ClusterIP
  port: 8080
  annotations: {}
  labels: {}
```

The Service exposes the Agent HTTP port inside the cluster.

## Prometheus integration

Prometheus Operator integration:

```yaml
serviceMonitor:
  enabled: false
  labels: {}
  annotations: {}
  interval: 30s
  scrapeTimeout: 10s
  honorLabels: true
  metricRelabelings: []
  relabelings: []
```

Annotation-based Prometheus discovery:

```yaml
prometheus:
  scrapeAnnotations:
    enabled: false
    target: service
    path: /metrics
    port: "8080"
```

`serviceMonitor.honorLabels=true` preserves KBeacon metric labels such as `namespace`, `secret_name`, and `workload_name`.

## RBAC

```yaml
rbac:
  create: true
  scope: cluster
  extraRules: []
```

Supported scopes:

| Scope | Rendered objects |
| --- | --- |
| `cluster` | `ClusterRole` and `ClusterRoleBinding` |
| `namespace` | `Role` and `RoleBinding` |

RBAC rules are generated from `resourcesToWatch`. Disabled resources do not receive RBAC rules.

## Service account

```yaml
serviceAccount:
  create: true
  name: ""
  annotations: {}
```

Set `serviceAccount.create=false` and `serviceAccount.name` when using an externally managed ServiceAccount.

## Grafana dashboards

```yaml
dashboards:
  enabled: false
  labels:
    grafana_dashboard: "1"
  annotations: {}
```

When enabled, the chart renders dashboard ConfigMaps from `charts/kbeacon/dashboards/`.

Included dashboards:

- `KBeacon / Cluster Overview`
- `KBeacon / Secret Dependency Map`
- `KBeacon / Team Overview`
- `KBeacon / Dependency Graph Explorer`

The Dependency Graph Explorer requires `metrics.edge.enabled=true`.

## NetworkPolicy

```yaml
networkPolicy:
  enabled: false
  ingress:
    from: []
```

When enabled, the chart renders an ingress-only NetworkPolicy for the Agent HTTP port.

## Pod and container security

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

## Resources

```yaml
resources:
  requests:
    cpu: 50m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

## Scheduling and extensibility

```yaml
nodeSelector: {}
tolerations: []
affinity: {}
priorityClassName: ""
podAnnotations: {}
podLabels: {}
extraArgs: []
extraEnv: []
extraVolumes: []
extraVolumeMounts: []
```

## External configuration

```yaml
config:
  create: true
  existingConfigMap: ""
```

Set `config.create=false` and `config.existingConfigMap` only when supplying an externally managed Agent config with the same schema as the chart-generated config.

### ServiceAccount imagePullSecrets fallback

KBeacon discovers Pod-level `spec.imagePullSecrets` when `discovery.includeImagePullSecrets=true`.

When a workload does not define Pod-level `imagePullSecrets`, KBeacon can use the workload ServiceAccount as a fallback and discover Secrets from `serviceAccount.imagePullSecrets`.

The fallback requires ServiceAccount watch access:

```yaml
resourcesToWatch:
  core:
    serviceAccounts: true
```

With `resourcesToWatch.core.serviceAccounts=false`, the chart omits ServiceAccount RBAC and the Agent cannot discover ServiceAccount image pull Secret fallbacks.

Pod-level `imagePullSecrets` take precedence. KBeacon does not add ServiceAccount fallback edges when the Pod spec already contains explicit image pull Secrets.

### Ingress TLS Secret discovery

KBeacon discovers TLS Secret references from networking.k8s.io/v1 Ingress resources when Ingress watching is enabled.

```yaml
resourcesToWatch:
  networking:
    ingresses: true
```

The chart renders read-only `get`, `list`, and `watch` RBAC for networking.k8s.io Ingress resources by default.

Disable Ingress watching when the cluster does not use Ingress TLS or when the Agent should not receive Ingress permissions:

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --set cluster.name=prod-eu-1 \
  --set resourcesToWatch.networking.ingresses=false
```

Ingress TLS edges use dependency source type `ingress.tls`.

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

replicaCount: 1

KBeacon v0.3.x is intentionally single-replica. Each Agent replica builds its own in-memory dependency graph. Running more than one replica without leader election can duplicate Prometheus metrics and expose replica-local API snapshots.

The chart validates `replicaCount=1` and rejects other values. High availability with leader election is tracked as future work.

## Projected Secret volumes

Kubernetes projected volumes can include Secret projections. KBeacon discovers these references from Pod specs and workload Pod templates.

Supported source path:

    spec.volumes[].projected.sources[].secret.name

KBeacon records these dependencies with source type `volumes.projected.sources.secret`. The dependency is namespace-local to the workload, matching Kubernetes Secret volume semantics.

## Privacy and redaction

Example values:

    privacy:
      redaction:
        secretKeys: true

`privacy.redaction.secretKeys=true` redacts Secret key names in dependency source paths returned by the Agent API. Secret names and namespaces remain visible because they are required for dependency analysis.

## Kind E2E smoke test

KBeacon includes a Kind-based end-to-end smoke test for the chart, RBAC, Kubernetes informers, projected Secret volume discovery, privacy redaction, and the read-only Agent API.

Run it locally when docker, kind, kubectl, helm, and python3 are available:

    make kind-e2e-smoke

The test builds a local kbeacon-agent:e2e image, loads it into a temporary Kind cluster, installs the Helm chart, creates a small workload graph, and verifies the Agent API.

## Supported resource matrix

The implemented Kubernetes resource and dependency source matrix is maintained in [`supported-resources.md`](supported-resources.md).

Use that page as the source of truth for what KBeacon watches today and what is only future roadmap scope.

For namespace-scoped RBAC, configure exactly one `discovery.namespaces.include` entry matching the release namespace. In that mode, the Agent uses namespace-scoped Kubernetes informers instead of cluster-wide list/watch calls.\n

## cert-manager Certificate discovery

Enable this optional watcher only when cert-manager CRDs are installed:

```bash
helm upgrade --install kbeacon ./charts/kbeacon   --namespace kbeacon-system   --create-namespace   --set cluster.name=prod-eu-1   --set resourcesToWatch.certManager.certificates=true
```

The chart adds read-only RBAC for `cert-manager.io` `certificates`, and the Agent discovers `spec.secretName` target Secrets.

## ExternalSecret discovery

Enable this optional watcher only when External Secrets Operator CRDs are installed:

~~~bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set resourcesToWatch.externalSecrets.externalSecrets=true
~~~

When enabled, the chart adds read-only RBAC for `external-secrets.io` `externalsecrets`, and the Agent discovers the Kubernetes Secret managed by each `ExternalSecret`.

KBeacon uses `spec.target.name` as the inferred target Secret name. If `spec.target.name` is omitted, KBeacon falls back to the `ExternalSecret` object `metadata.name`, matching the common External Secrets Operator default target Secret behavior.

ExternalSecret edges use dependency source type `external-secrets.externalsecret.spec.target.name`.

Leave this watcher disabled unless the `externalsecrets.external-secrets.io` CRD exists in the cluster.

## SecretProviderClass discovery

Enable this optional watcher only when Secrets Store CSI Driver CRDs are installed:

~~~bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set resourcesToWatch.secretsStore.secretProviderClasses=true
~~~

When enabled, the chart adds read-only RBAC for `secrets-store.csi.x-k8s.io` `secretproviderclasses`, and the Agent discovers synced Kubernetes Secrets listed in each `SecretProviderClass`.

KBeacon models every non-empty `spec.secretObjects[*].secretName` entry as a dependency edge to a Kubernetes Secret in the same namespace as the `SecretProviderClass`.

SecretProviderClass edges use dependency source type `secrets-store.csi.secretproviderclass.spec.secretObjects.secretName`.

KBeacon does not inspect external provider object names, provider payloads, mounted file contents, or Secret values.

Leave this watcher disabled unless the `secretproviderclasses.secrets-store.csi.x-k8s.io` CRD exists in the cluster.

## Strimzi KafkaConnector discovery

Enable this optional watcher only when Strimzi KafkaConnector CRDs are installed:

~~~bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set resourcesToWatch.strimzi.kafkaConnectors=true
~~~

When enabled, the chart adds read-only RBAC for `kafka.strimzi.io` `kafkaconnectors`, and the Agent watches `kafka.strimzi.io/v1beta2` `KafkaConnector` resources.

KBeacon parses string values under `spec.config` for Strimzi Kubernetes Config Provider Secret references:

- `${secrets:namespace/name:key}`
- `${secrets:name:key}`

The second form uses the `KafkaConnector` namespace.

Strimzi KafkaConnector inferred edges use dependency source type `strimzi.kafkaconnector.spec.config.secrets`.

KBeacon does not call Kafka Connect REST APIs, inspect connector plugin payloads, read provider systems, or read Kubernetes Secret values.

Leave this watcher disabled unless the `kafkaconnectors.kafka.strimzi.io` CRD exists in the cluster.

## Confluent / Kafka Connect Connector discovery

Enable this optional watcher only when Confluent for Kubernetes Connector CRDs are installed:

~~~bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set resourcesToWatch.confluent.connectors=true
~~~

When enabled, the chart adds read-only RBAC for `platform.confluent.io` `connectors`, and the Agent watches `platform.confluent.io/v1beta1` `Connector` resources.

KBeacon infers Connector Secret dependencies from:

1. `spec.connectRest.authentication.*.secretRef`
2. string values under `spec.configs` that use `${file:/mnt/secrets/<secret>/...:key}` style mounted Secret references.

Connect REST authentication edges use dependency source type `confluent.connector.spec.connectRest.authentication.secretRef`.

Mounted Secret file edges use dependency source type `confluent.connector.spec.configs.file.mountedSecret`.

KBeacon does not call Kafka Connect REST APIs, inspect connector plugin payloads, read mounted file contents, or read Kubernetes Secret values.

Leave this watcher disabled unless the `connectors.platform.confluent.io` CRD exists in the cluster.

`resourcesToWatch.apps.replicaSets=true` starts a read-only ReplicaSet informer used only for owner resolution. KBeacon uses it to map ReplicaSet-owned Pods back to their Deployment when the Deployment is also watched. ReplicaSets are not emitted as primary workload nodes, and KBeacon does not infer Secret dependencies from ReplicaSet objects directly.
