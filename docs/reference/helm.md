# KBeacon Helm Reference

KBeacon deploys as one lightweight Deployment and one Service.

The chart does not install KBeacon CRDs, an operator, admission webhooks, databases, queues, or a UI.

## Minimal install

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1

## Local Minikube install

For local in-cluster development use:

    ./hack/local-dev/deploy-incluster-minikube.sh

The helper script builds `kbeacon-agent:dev` in the Minikube Docker daemon and installs this chart with:

    hack/local-dev/kbeacon-minikube-values.yaml

## Low-privilege mode

Some organizations do not allow an observability agent to `get`, `list`, or `watch` Kubernetes Secrets. KBeacon can still discover workload references without Secret object access.

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set resourcesToWatch.core.secrets=false
```

In this mode:

- the chart does not render Secret RBAC rules;
- the Agent does not start the Secret informer;
- workload-to-Secret edges are still discovered from Pod specs and explicit annotations;
- referenced Secrets are represented with `exists=false`;
- dependency edges have `resolved=false`;
- Secret metadata from Secret annotations, Secret type, change timestamps, and change counters are unavailable.

This mode is useful when the main requirement is blast-radius visibility and the cluster security model does not permit Secret reads.

## Prometheus Operator ServiceMonitor

Enable the ServiceMonitor only if Prometheus Operator CRDs are installed.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set serviceMonitor.enabled=true \
      --set serviceMonitor.labels.release=kube-prometheus-stack

## Standard Prometheus scrape target

Without Prometheus Operator, scrape the Service directly:

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

## Key values

### cluster

    cluster:
      name: prod-eu-1
      environment: prod
      region: eu

`cluster.name` is required for normal Helm installs.

### image

    image:
      repository: ghcr.io/memoliyasti/kbeacon
      tag: "0.1.2"
      digest: ""
      pullPolicy: IfNotPresent

For Minikube local image builds:

    image:
      repository: kbeacon-agent
      tag: dev
      pullPolicy: IfNotPresent

For private GHCR packages, create an image pull Secret and pass:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.1.2 \
      --set 'imagePullSecrets[0].name=ghcr-pull-secret'

### discovery.namespaces

    discovery:
      namespaces:
        include: []
        exclude:
          - kube-system
          - kube-public
          - kube-node-lease

Behavior:

- `include: []` means all namespaces are eligible unless excluded.
- non-empty `include` acts as an allow-list.
- `exclude` overrides `include`.

### discovery.defaultMode

    discovery:
      defaultMode: hybrid

Supported values: `infer`, `explicit`, `hybrid`, `disabled`.

### discovery.includeImagePullSecrets

    discovery:
      includeImagePullSecrets: true

When enabled, the Agent discovers dependencies from `spec.imagePullSecrets`.

### discovery.reconcile.debounce

    discovery:
      reconcile:
        debounce: 250ms

Debounces informer event bursts before rebuilding the dependency graph.

### resourcesToWatch

The Agent can enable or disable implemented resource informers from config.

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

Disabled resources appear in `/readyz` as:

    {
      "resource": "Pod",
      "synced": true,
      "optional": true,
      "reason": "disabled"
    }

Disabled resources are not emitted in `kbeacon_cache_sync_status`.

### metrics

    metrics:
      runtime:
        enabled: true

The current implementation always exposes graph metrics. Runtime collectors and recorder metrics are controlled by `metrics.runtime.enabled`.

### dashboards

    dashboards:
      enabled: false
      labels:
        grafana_dashboard: "1"

When enabled, the chart renders dashboard ConfigMaps from:

    charts/kbeacon/dashboards/

### rbac

    rbac:
      create: true
      scope: cluster

Recommended production mode is cluster-scoped read-only RBAC.

Namespace-scoped example:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace payments \
      --set cluster.name=prod-eu-1 \
      --set rbac.scope=namespace \
      --set discovery.namespaces.include='{payments}'

## Validation

Render chart:

    helm template kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --set cluster.name=minikube

Install chart:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=minikube
