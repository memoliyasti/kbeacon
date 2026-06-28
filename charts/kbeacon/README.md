# KBeacon Helm Chart

This chart deploys one KBeacon Agent per Kubernetes cluster.

It intentionally does not install KBeacon CRDs, an operator, admission webhooks, databases, queues, or a UI.

## Install

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1
```

## Local Minikube install

Build the local image into the Minikube Docker daemon and install the chart:

```bash
./hack/local-dev/deploy-incluster-minikube.sh
```

Configure the local Prometheus chart to scrape KBeacon:

```bash
./hack/local-dev/configure-prometheus-incluster.sh
```

Run smoke tests:

```bash
./hack/local-dev/smoke-incluster.sh
```

## Low-privilege mode

Disable Secret watching when cluster policy does not allow the Agent ServiceAccount to read Kubernetes Secrets:

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set resourcesToWatch.core.secrets=false
```

The Agent still discovers workload references, but referenced Secrets are marked `exists=false` and dependency edges are marked `resolved=false`.

## ServiceMonitor

Enable only when Prometheus Operator CRDs are installed.

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set serviceMonitor.enabled=true \
  --set serviceMonitor.labels.release=kube-prometheus-stack
```

## Dashboard ConfigMaps

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --set cluster.name=prod-eu-1 \
  --set dashboards.enabled=true
```

Dashboards are rendered from:

```text
charts/kbeacon/dashboards/
```

## Resource watcher enablement

```yaml
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
```

Disabled resources are marked optional in `/readyz`.

## Namespace filtering

```yaml
discovery:
  namespaces:
    include:
      - payments
    exclude:
      - kube-system
      - kube-public
      - kube-node-lease
```

## Values

See [`values.yaml`](values.yaml) for the complete values file.
