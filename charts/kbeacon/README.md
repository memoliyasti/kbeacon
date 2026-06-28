
# KBeacon Helm Chart

This chart deploys one KBeacon Agent per Kubernetes cluster.

It intentionally does not install KBeacon CRDs, an operator, admission webhooks, databases, queues, or a UI.

## Install

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1

## Low-privilege mode

Disable Secret watching when cluster policy does not allow the Agent ServiceAccount to read Kubernetes Secrets.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set resourcesToWatch.core.secrets=false

The Agent still discovers workload references, but referenced Secrets are marked `exists=false` and dependency edges are marked `resolved=false`.

## Namespace-scoped mode

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace payments \
      --set cluster.name=prod-eu-1 \
      --set rbac.scope=namespace \
      --set discovery.namespaces.include="{payments}"

## ServiceMonitor

Enable only when Prometheus Operator CRDs are installed.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set serviceMonitor.enabled=true \
      --set serviceMonitor.labels.release=kube-prometheus-stack

## Dashboard ConfigMaps

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --set cluster.name=prod-eu-1 \
      --set dashboards.enabled=true

Dashboards are rendered from `charts/kbeacon/dashboards/`.

## Metrics cardinality guard

Disable detailed edge metrics when Prometheus cardinality is a concern.

    metrics:
      edge:
        enabled: false

The Agent still emits aggregate metrics and the REST API still exposes dependency edges.

## Resource watcher enablement

    resourcesToWatch:
      core:
        secrets: true
        pods: false
      apps:
        deployments: true
        statefulSets: false
        daemonSets: false
      batch:
        jobs: false
        cronJobs: false

Disabled resources are marked optional in `/readyz`.

## Values

See `values.yaml` and `docs/reference/helm.md`.
