# KBeacon Helm Chart

This chart deploys one read-only KBeacon Agent per Kubernetes cluster.

KBeacon observes workload metadata and Secret references, builds an in-memory dependency graph, exposes Prometheus metrics, and optionally renders Grafana dashboard ConfigMaps.

The chart does not install CRDs, operators, admission webhooks, databases, queues, or a custom UI.

## Required value

`cluster.name` is required. It is emitted as the logical cluster identity in metrics, API responses, and generated Agent configuration.

```yaml
cluster:
  name: prod-eu-1
```

## Install from the public chart repository

```bash
helm repo add kbeacon-release https://memoliyasti.github.io/kbeacon/charts
helm repo update kbeacon-release

VERSION="$(helm search repo kbeacon-release/kbeacon --versions | awk 'NR==2 {print $2}')"

helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --wait \
  --timeout 10m
```

## Image

The default image is published to GitHub Container Registry. Published chart packages set `values.yaml` `image.tag` to the matching chart application version:

```yaml
image:
  repository: ghcr.io/memoliyasti/kbeacon
  tag: "<release version>"
  pullPolicy: IfNotPresent
```

Use `image.digest` for immutable production deployments. When `image.digest` is set, the chart renders `repository@digest` instead of `repository:tag`.

## Low-privilege mode

Disable Secret watching when cluster policy does not allow the Agent ServiceAccount to read Kubernetes Secret objects:

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

The Agent still discovers workload references, but referenced Secrets are marked `exists=false` and dependency edges are marked `resolved=false`.

## Discovery scope

KBeacon can run cluster-wide or namespace-scoped.

Cluster-wide mode is the default:

```yaml
rbac:
  scope: cluster
```

Namespace-scoped mode:

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

## Prometheus integration

Enable `ServiceMonitor` only when Prometheus Operator CRDs are installed:

```bash
helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set serviceMonitor.enabled=true \
  --wait \
  --timeout 10m
```

Enable scrape annotations for Prometheus setups that discover Services through `prometheus.io/*` annotations:

```bash
helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set prometheus.scrapeAnnotations.enabled=true \
  --wait \
  --timeout 10m
```

## Grafana dashboard ConfigMaps

```bash
helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=prod-eu-1 \
  --set dashboards.enabled=true \
  --wait \
  --timeout 10m
```

Dashboards are rendered from `charts/kbeacon/dashboards/`.

## Metrics cardinality guard

Disable detailed edge metrics when Prometheus cardinality is a concern:

```yaml
metrics:
  edge:
    enabled: false
```

The Agent still emits aggregate metrics and the REST API still exposes dependency edges.

## Optional resource watchers

KBeacon can watch optional resources when the matching CRDs are already installed in the cluster:

```yaml
resourcesToWatch:
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

The corresponding dependency source types are `ingress.tls`, `cert-manager.certificate.spec.secretName`, `externalsecret.spec.target.name`, `secretproviderclass.spec.secretObjects.secretName`, `strimzi.configProvider.secret`, and `confluent.connector.mountedSecret`.
## Supported resources

For the full supported watcher matrix and dependency source paths, see [Supported resources](../../docs/reference/supported-resources.md).


## Values

See `values.yaml` and `docs/reference/helm.md`.
