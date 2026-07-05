# Configuration

KBeacon is configured through Helm values. The chart renders those values into the Agent ConfigMap and Kubernetes deployment resources.

This page describes the main configuration areas and the operational trade-offs behind them.

For the exhaustive values contract, use `charts/kbeacon/values.yaml` and `docs/reference/helm.md`.

## Configuration model

KBeacon has two configuration layers.

| Layer | Purpose | Source |
| --- | --- | --- |
| Helm chart values | Kubernetes deployment, RBAC, Service, ServiceMonitor, dashboard ConfigMaps, security context, resources, scheduling. | `charts/kbeacon/values.yaml` |
| Agent config | Cluster identity, discovery behavior, resource watchers, metrics behavior. | chart-rendered `config.yaml` |

The chart-managed ConfigMap is enabled by default.

```yaml
config:
  create: true
  existingConfigMap: ""
```

Use `config.existingConfigMap` only when an external configuration pipeline owns the Agent config schema.

## Cluster identity

`cluster.name` is required and should be stable.

```yaml
cluster:
  name: prod-eu-1
  environment: prod
  region: eu
```

The cluster name is used in Prometheus labels, Agent API responses, dashboard variables, and generated Agent configuration.

`cluster.environment` and `cluster.region` are optional metadata fields reserved for consistent platform configuration.

## Discovery behavior

```yaml
discovery:
  defaultMode: hybrid
  includeImagePullSecrets: true
  includeInitContainers: true
  includeEphemeralContainers: true
  readPodTemplateAnnotations: true
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

`hybrid` is the recommended default because it supports standard Kubernetes references and explicit overrides.

## Namespace selection

```yaml
discovery:
  namespaces:
    include: []
    exclude:
      - kube-system
      - kube-public
      - kube-node-lease
```

Namespace rules:

- `include: []` means all namespaces are eligible unless excluded;
- a non-empty `include` list acts as an allow-list;
- `exclude` takes precedence over `include`.

Use namespace selection to keep discovery aligned with tenancy and platform boundaries.

## Metadata label fallback

KBeacon can map existing workload labels into ownership and classification fields.

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

Precedence:

1. KBeacon annotations.
2. Workload object labels.
3. Pod template labels when pod template metadata is read.

Use annotations for explicit overrides. Use label fallback to adopt KBeacon without adding metadata-only annotations to every workload.

## Resource watchers

KBeacon starts informers only for enabled resources.

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

| Value path | Kubernetes resource |
| --- | --- |
| `resourcesToWatch.core.secrets` | `Secret` |
| `resourcesToWatch.core.serviceAccounts` | `ServiceAccount` |
| `resourcesToWatch.core.pods` | `Pod` |
| `resourcesToWatch.apps.deployments` | `Deployment` |
| `resourcesToWatch.apps.statefulSets` | `StatefulSet` |
| `resourcesToWatch.apps.daemonSets` | `DaemonSet` |
| `resourcesToWatch.batch.jobs` | `Job` |
| `resourcesToWatch.batch.cronJobs` | `CronJob` |
| `resourcesToWatch.networking.ingresses` | `Ingress` |
| `resourcesToWatch.certManager.certificates` | `Certificate` |
| `resourcesToWatch.externalSecrets.externalSecrets` | `ExternalSecret` |
| `resourcesToWatch.secretsStore.secretProviderClasses` | `SecretProviderClass` |

Disabled resources are not watched and are represented as optional in readiness status.

## Low-privilege mode

Disable Secret object watching when the Agent must not read Kubernetes Secret objects.

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

## Metrics configuration

```yaml
metrics:
  edge:
    enabled: true
  runtime:
    enabled: true
```

`metrics.edge.enabled=false` disables the high-cardinality `kbeacon_dependency_edges` metric family.

When edge metrics are disabled:

- aggregate graph metrics remain available;
- the read-only Agent API remains available;
- edge-level Grafana Node Graph panels do not show dependency edges.

`metrics.runtime.enabled=false` disables runtime collector and recorder metrics. Graph metrics remain available.

## Prometheus integration

KBeacon exposes Prometheus metrics at `/metrics` on the Agent HTTP port.

Prometheus Operator profile:

```yaml
serviceMonitor:
  enabled: true
  labels: {}
  annotations: {}
  interval: 30s
  scrapeTimeout: 10s
  honorLabels: true
  metricRelabelings: []
  relabelings: []
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

`serviceMonitor.honorLabels=true` preserves KBeacon metric labels such as `namespace`, `secret_name`, and `workload_name`.

## Grafana dashboards

```yaml
dashboards:
  enabled: false
  labels:
    grafana_dashboard: "1"
  annotations: {}
```

Enable dashboards only when Grafana is configured to discover dashboard ConfigMaps by label.

The Dependency Graph Explorer requires `metrics.edge.enabled=true` because it is powered by `kbeacon_dependency_edges`.

## RBAC

```yaml
rbac:
  create: true
  scope: cluster
  extraRules: []
```

`rbac.scope` supports:

| Scope | Rendered resources | Use case |
| --- | --- | --- |
| `cluster` | `ClusterRole` and `ClusterRoleBinding` | one Agent observes the cluster |
| `namespace` | `Role` and `RoleBinding` | one Agent observes a namespace or tenant slice |

RBAC rules are generated from `resourcesToWatch`. Disabled resources do not receive watch permissions.

## Pod and container security

The chart defaults to a non-root, read-only container posture.

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

Secret names and dependency metadata may still be sensitive. Protect metrics, dashboards, logs, and API access according to your cluster security model.

## Scheduling and extension points

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

Use these values to integrate KBeacon with platform scheduling, policy, and runtime conventions without changing the chart templates.

## Validation

Configuration changes should be validated through chart rendering and repository checks.

Recommended checks:

- default chart render;
- low-privilege chart render;
- namespace-scoped RBAC render;
- dashboard JSON validation;
- Prometheus rule validation;
- `make validate-ci`.

## ServiceAccount imagePullSecrets fallback

Kubernetes Pods can reference registry pull Secrets directly through `spec.imagePullSecrets` or indirectly through their ServiceAccount.

KBeacon handles both patterns when inferred image pull Secret discovery and ServiceAccount watching are enabled.

```yaml
discovery:
  includeImagePullSecrets: true

resourcesToWatch:
  core:
    serviceAccounts: true
```

Fallback behavior:

- Pod-level `spec.imagePullSecrets` are discovered first.
- If Pod-level `imagePullSecrets` are absent, KBeacon looks up the workload ServiceAccount.
- Secrets from `serviceAccount.imagePullSecrets` are represented as inferred dependency edges.
- The dependency source type is `serviceAccount.imagePullSecrets`.

## Ingress TLS Secret discovery

Ingress TLS discovery is controlled by the networking resource watcher.

```yaml
resourcesToWatch:
  networking:
    ingresses: true
```

When enabled, KBeacon watches networking.k8s.io/v1 Ingress resources and records each `spec.tls[].secretName` reference as an inferred Secret dependency.

Disable it when Ingress TLS is out of scope:

```yaml
resourcesToWatch:
  networking:
    ingresses: false
```

## Privacy and redaction

Use `privacy.redaction.secretKeys=true` when Secret key names should not appear in Agent API source paths.

Example:

    privacy:
      redaction:
        secretKeys: true

This changes source paths such as `env[DB_PASSWORD].valueFrom.secretKeyRef[payments-db#password]` to `env[DB_PASSWORD].valueFrom.secretKeyRef[payments-db#<redacted>]`.

## cert-manager Certificate watcher

`resourcesToWatch.certManager.certificates=false` by default. Set it to `true` only when cert-manager CRDs are installed. When enabled, KBeacon watches `cert-manager.io/v1` `Certificate` resources and adds dependency edges from each Certificate to `spec.secretName`.

## ExternalSecret watcher

`resourcesToWatch.externalSecrets.externalSecrets=false` by default.

Set it to `true` only when External Secrets Operator CRDs are installed:

~~~yaml
resourcesToWatch:
  externalSecrets:
    externalSecrets: true
~~~

When enabled, KBeacon watches `external-secrets.io/v1` `ExternalSecret` resources and adds dependency edges from each `ExternalSecret` to its target Kubernetes Secret.

Target Secret resolution:

1. `spec.target.name` when it is set.
2. `metadata.name` when `spec.target.name` is omitted.

The dependency source type is `external-secrets.externalsecret.spec.target.name`.

The Helm chart renders read-only `get`, `list`, and `watch` RBAC for `external-secrets.io` `externalsecrets` only when this watcher is enabled.

## SecretProviderClass watcher

`resourcesToWatch.secretsStore.secretProviderClasses=false` by default.

Set it to `true` only when Secrets Store CSI Driver CRDs are installed:

~~~yaml
resourcesToWatch:
  secretsStore:
    secretProviderClasses: true
~~~

When enabled, KBeacon watches `secrets-store.csi.x-k8s.io/v1` `SecretProviderClass` resources and adds dependency edges from each `SecretProviderClass` to its synced Kubernetes Secrets.

Target Secret resolution:

1. Iterate over `spec.secretObjects`.
2. Read every non-empty `secretName`.
3. Model each target as a Kubernetes Secret in the same namespace as the `SecretProviderClass`.

The dependency source type is `secrets-store.csi.secretproviderclass.spec.secretObjects.secretName`.

The Helm chart renders read-only `get`, `list`, and `watch` RBAC for `secrets-store.csi.x-k8s.io` `secretproviderclasses` only when this watcher is enabled.

KBeacon does not read external provider object values, mounted file contents, or Kubernetes Secret data.
