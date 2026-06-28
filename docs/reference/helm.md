# KBeacon Helm Reference

KBeacon deploys as one lightweight Deployment and one Service.

The chart does **not** install KBeacon CRDs, an operator, admission webhooks, databases, queues, or a UI.

## Minimal install

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1
```

## Local Minikube install

For local in-cluster development use:

```bash
./hack/local-dev/deploy-incluster-minikube.sh
```

The helper script builds `kbeacon-agent:dev` in the Minikube Docker daemon and installs this chart with:

```text
hack/local-dev/kbeacon-minikube-values.yaml
```

## Prometheus Operator ServiceMonitor

Enable the ServiceMonitor only if Prometheus Operator CRDs are installed.

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set serviceMonitor.enabled=true \
  --set serviceMonitor.labels.release=kube-prometheus-stack
```

## Standard Prometheus scrape target

Without Prometheus Operator, scrape the Service directly:

```yaml
scrape_configs:
  - job_name: kbeacon-agent
    honor_labels: true
    metrics_path: /metrics
    static_configs:
      - targets:
          - kbeacon.kbeacon-system.svc.cluster.local:8080
        labels:
          cluster: prod-eu-1
          app: kbeacon
          component: agent
```

## Key values

### `cluster`

```yaml
cluster:
  name: prod-eu-1
  environment: prod
  region: eu
```

`cluster.name` is required for normal Helm installs.

### `image`

```yaml
image:
  repository: ghcr.io/memoliyasti/kbeacon
  tag: "0.1.0"
  pullPolicy: IfNotPresent
```

For Minikube local image builds:

```yaml
image:
  repository: kbeacon-agent
  tag: dev
  pullPolicy: IfNotPresent
```

For personal GHCR releases:

```yaml
image:
  repository: ghcr.io/<github-user-or-org>/kbeacon-agent
  tag: v0.1.0
```

### `discovery.namespaces`

```yaml
discovery:
  namespaces:
    include: []
    exclude:
      - kube-system
      - kube-public
      - kube-node-lease
```

Behavior:

- `include: []` means all namespaces are eligible unless excluded.
- non-empty `include` acts as an allow-list.
- `exclude` overrides `include`.

### `discovery.defaultMode`

```yaml
discovery:
  defaultMode: hybrid
```

Supported values: `infer`, `explicit`, `hybrid`, `disabled`.

### `discovery.includeImagePullSecrets`

```yaml
discovery:
  includeImagePullSecrets: true
```

When enabled, the Agent discovers dependencies from `spec.imagePullSecrets`.

### `discovery.reconcile.debounce`

```yaml
discovery:
  reconcile:
    debounce: 250ms
```

Debounces informer event bursts before rebuilding the dependency graph.

### `resourcesToWatch`

The Agent can enable or disable resource informers from config.

```yaml
resourcesToWatch:
  core:
    secrets: true
    pods: true
  apps:
    deployments: true
    statefulSets: true
    daemonSets: true
    replicaSets: true
  batch:
    jobs: true
    cronJobs: true
```

Currently implemented watchers:

| Value path | Implemented |
| --- | --- |
| `resourcesToWatch.core.secrets` | yes |
| `resourcesToWatch.core.pods` | yes |
| `resourcesToWatch.apps.deployments` | yes |
| `resourcesToWatch.apps.statefulSets` | yes |
| `resourcesToWatch.apps.daemonSets` | yes |
| `resourcesToWatch.batch.jobs` | yes |
| `resourcesToWatch.batch.cronJobs` | yes |
| `resourcesToWatch.apps.replicaSets` | reserved for owner-resolution improvements |

Disabled resources appear in `/readyz` as:

```json
{
  "resource": "Pod",
  "synced": true,
  "optional": true,
  "reason": "disabled"
}
```

Disabled resources are not emitted in `kbeacon_cache_sync_status`.

### `metrics`

```yaml
metrics:
  enabled: true
  edge:
    enabled: true
  runtime:
    enabled: true
```

The current implementation always registers graph collectors. Runtime collectors are controlled by `metrics.runtime.enabled`.

### `dashboards`

```yaml
dashboards:
  enabled: false
  labels:
    grafana_dashboard: "1"
```

When enabled, the chart renders dashboard ConfigMaps from:

```text
charts/kbeacon/dashboards/
```

### `rbac`

```yaml
rbac:
  create: true
  scope: cluster
```

Recommended production mode is cluster-scoped read-only RBAC.

Namespace-scoped example:

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace payments \
  --set cluster.name=prod-eu-1 \
  --set rbac.scope=namespace \
  --set discovery.namespaces.include='{payments}'
```

## Local development values

See:

```text
hack/local-dev/kbeacon-minikube-values.yaml
hack/local-dev/prometheus-kbeacon-incluster-values.yaml
```

## Validation

Render chart:

```bash
helm template kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --set cluster.name=minikube
```

Install chart:

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=minikube
```

Check rollout:

```bash
kubectl -n kbeacon-system rollout status deploy/kbeacon
kubectl -n kbeacon-system get pods
kubectl -n kbeacon-system logs deploy/kbeacon --tail=100
```
