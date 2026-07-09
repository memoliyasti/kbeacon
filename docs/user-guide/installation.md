# Installation

KBeacon is installed with Helm and verified with the kube-native `kbeacon` CLI.

## Recommended install path

Use the public Helm chart repository:

```bash
helm repo add kbeacon-release https://memoliyasti.github.io/kbeacon/charts
helm repo update kbeacon-release

helm search repo kbeacon-release/kbeacon --versions | head
```

Install the latest published chart:

```bash
VERSION="$(helm search repo kbeacon-release/kbeacon --versions | awk 'NR==2 {print $2}')"

helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=my-cluster \
  --set dashboards.enabled=true \
  --set serviceMonitor.enabled=true \
  --set prometheus.scrapeAnnotations.enabled=true \
  --wait \
  --timeout 10m
```

For production, pin `VERSION` to a reviewed chart version instead of always selecting the newest entry.

## Verify the Agent

```bash
kubectl -n kbeacon-system rollout status deploy/kbeacon --timeout=5m
kubectl -n kbeacon-system get deploy,pod,svc,cm,servicemonitor
kubectl -n kbeacon-system logs deploy/kbeacon --tail=100

kbeacon config set namespace kbeacon-system
kbeacon ready
kbeacon get config
kbeacon get secrets --limit 100
kbeacon get workloads --limit 100
```

The CLI uses the current kubeconfig context and Kubernetes API server Service proxy by default.

## Low-privilege install

Use this mode when the KBeacon ServiceAccount must not read Kubernetes Secret objects:

```bash
VERSION="$(helm search repo kbeacon-release/kbeacon --versions | awk 'NR==2 {print $2}')"

helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=my-cluster \
  --set resourcesToWatch.core.secrets=false \
  --wait \
  --timeout 10m
```

KBeacon still discovers workload-to-Secret references from workload specs and explicit annotations. Referenced Secrets are marked as unobservable because the Agent cannot confirm Secret object metadata.

## Prometheus and Grafana integration

Enable Prometheus Operator `ServiceMonitor` support when the CRDs are installed:

```bash
helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=my-cluster \
  --set serviceMonitor.enabled=true \
  --set dashboards.enabled=true \
  --wait \
  --timeout 10m
```

Enable scrape annotations for Prometheus setups that discover Services through `prometheus.io/*` annotations:

```bash
helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=my-cluster \
  --set prometheus.scrapeAnnotations.enabled=true \
  --wait \
  --timeout 10m
```

## Private registry or fork

The official GHCR image for this repository is intended to be public. Use `imagePullSecrets` only for private forks or private registries.

```bash
kubectl create namespace kbeacon-system --dry-run=client -o yaml | kubectl apply -f -

kubectl -n kbeacon-system create secret docker-registry ghcr-pull-secret \
  --docker-server=ghcr.io \
  --docker-username=<github-username> \
  --docker-password=<token-with-read-packages> \
  --docker-email=<email> \
  --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=my-cluster \
  --set 'imagePullSecrets[0].name=ghcr-pull-secret' \
  --wait \
  --timeout 10m
```

## Digest pinning

```bash
helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=my-cluster \
  --set image.repository=ghcr.io/memoliyasti/kbeacon \
  --set image.digest=sha256:<digest> \
  --wait \
  --timeout 10m
```

When `image.digest` is set, the chart renders `repository@digest` instead of `repository:tag`.

## Local development from source

For local chart and image development, build into Minikube or Kind and install from `./charts/kbeacon`. The public Helm repository remains the recommended user-facing install path.

```bash
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --create-namespace \
  --set cluster.name=local-dev \
  --wait \
  --timeout 10m
```
