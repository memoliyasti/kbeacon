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
  tag: "0.2.3"
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
    pods: true
  apps:
    deployments: true
    statefulSets: true
    daemonSets: true
  batch:
    jobs: true
    cronJobs: true
```

Implemented watcher values:

| Value path | Resource |
| --- | --- |
| `resourcesToWatch.core.secrets` | `Secret` |
| `resourcesToWatch.core.pods` | `Pod` |
| `resourcesToWatch.apps.deployments` | `Deployment` |
| `resourcesToWatch.apps.statefulSets` | `StatefulSet` |
| `resourcesToWatch.apps.daemonSets` | `DaemonSet` |
| `resourcesToWatch.batch.jobs` | `Job` |
| `resourcesToWatch.batch.cronJobs` | `CronJob` |

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
