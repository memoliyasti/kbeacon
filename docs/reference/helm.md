
# KBeacon Helm Reference

KBeacon deploys as one lightweight Deployment and one Service.

The chart does not install KBeacon CRDs, an operator, admission webhooks, databases, queues, or a UI.

## Minimal install

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1

## Private GHCR package

If the GHCR package is private, create an image pull Secret and pass it to the chart.

    kubectl create namespace kbeacon-system --dry-run=client -o yaml | kubectl apply -f -

    read -rsp "GHCR read:packages token: " GHCR_TOKEN
    echo

    kubectl -n kbeacon-system create secret docker-registry ghcr-pull-secret \
      --docker-server=ghcr.io \
      --docker-username=<github-username> \
      --docker-password="${GHCR_TOKEN}" \
      --docker-email=<email> \
      --dry-run=client -o yaml | kubectl apply -f -

    unset GHCR_TOKEN

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.2.1 \
      --set "imagePullSecrets[0].name=ghcr-pull-secret"

## Low-privilege mode

Disable Secret object watching when cluster policy does not allow the Agent ServiceAccount to read Kubernetes Secrets.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set resourcesToWatch.core.secrets=false

In this mode:

- the chart does not render Secret RBAC rules;
- the Agent does not start the Secret informer;
- workload-to-Secret edges are still discovered from Pod specs and annotations;
- referenced Secrets are represented with `exists=false`;
- dependency edges have `resolved=false`;
- Secret type, Secret annotations, change timestamps, and change counters are unavailable.

## Namespace-scoped install

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace payments \
      --set cluster.name=prod-eu-1 \
      --set rbac.scope=namespace \
      --set discovery.namespaces.include="{payments}"

`rbac.scope` must be either `cluster` or `namespace`.

## Prometheus Operator ServiceMonitor

Enable the ServiceMonitor only if Prometheus Operator CRDs are installed.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set serviceMonitor.enabled=true \
      --set serviceMonitor.labels.release=kube-prometheus-stack

## Standard Prometheus scrape target

Without Prometheus Operator, scrape the Service directly.

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

## Implemented values

### cluster

    cluster:
      name: prod-eu-1
      environment: prod
      region: eu

`cluster.name` is required. `environment` and `region` are metadata fields reserved for configuration consistency.

### image

    image:
      repository: ghcr.io/memoliyasti/kbeacon
      tag: "0.2.1"
      digest: ""
      pullPolicy: IfNotPresent

When `image.digest` is set, the chart renders `repository@digest` instead of `repository:tag`.

### discovery

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

Supported discovery modes: `infer`, `explicit`, `hybrid`, `disabled`.

Namespace behavior:

- `include: []` means all namespaces are eligible unless excluded.
- non-empty `include` acts as an allow-list.
- `exclude` overrides `include`.

### resourcesToWatch

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

Implemented watchers:

| Value path | Implemented |
| --- | --- |
| `resourcesToWatch.core.secrets` | yes |
| `resourcesToWatch.core.pods` | yes |
| `resourcesToWatch.apps.deployments` | yes |
| `resourcesToWatch.apps.statefulSets` | yes |
| `resourcesToWatch.apps.daemonSets` | yes |
| `resourcesToWatch.batch.jobs` | yes |
| `resourcesToWatch.batch.cronJobs` | yes |

Disabled resources appear in `/readyz` as optional and are not emitted in `kbeacon_cache_sync_status`.

### metrics

    metrics:
      edge:
        enabled: true
      runtime:
        enabled: true

`metrics.edge.enabled=false` disables only the high-cardinality `kbeacon_dependency_edges` metric family. Aggregate graph metrics and the Agent API remain available.

`metrics.runtime.enabled=false` disables runtime recorder and runtime collector metrics. Graph metrics remain available.

### dashboards

    dashboards:
      enabled: false
      labels:
        grafana_dashboard: "1"

When enabled, the chart renders dashboard ConfigMaps from `charts/kbeacon/dashboards/`.

### config

    config:
      create: true
      existingConfigMap: ""

Set `config.create=false` and `config.existingConfigMap=<name>` only when supplying an externally managed Agent config with the same schema as the chart-generated config.

## Validation

Render default chart:

    helm template kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --set cluster.name=ci

Render low-privilege mode:

    helm template kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --set cluster.name=ci \
      --set resourcesToWatch.core.secrets=false

Render namespace-scoped RBAC:

    helm template kbeacon ./charts/kbeacon \
      --namespace payments \
      --set cluster.name=ci \
      --set rbac.scope=namespace \
      --set discovery.namespaces.include="{payments}"
