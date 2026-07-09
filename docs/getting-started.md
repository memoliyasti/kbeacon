# Getting started

This guide installs the latest published KBeacon Helm chart and verifies it with the kube-native `kbeacon` CLI.

## Prerequisites

```bash
kubectl config current-context
helm version
kbeacon version
jq --version
```

## Add the public Helm repository

```bash
helm repo add kbeacon-release https://memoliyasti.github.io/kbeacon/charts
helm repo update kbeacon-release

helm search repo kbeacon-release/kbeacon --versions | head
```

## Install KBeacon

Use the latest chart version from the public repository:

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

For production, pin `VERSION` explicitly after reviewing the available chart versions.

## Verify

```bash
kubectl -n kbeacon-system rollout status deploy/kbeacon --timeout=5m
kubectl -n kbeacon-system get deploy,pod,svc,cm,servicemonitor

kbeacon config set namespace kbeacon-system
kbeacon ready
kbeacon get config
```

The CLI uses the current kubeconfig context and Kubernetes API server Service proxy by default. No `kubectl port-forward` is required for normal CLI use.

## Try the demo

```bash
./examples/demo-blast-radius/run.sh apply

kbeacon impact --format json payments payments-db | jq ".data.summary"
kbeacon get dependency-map --limit 500 | jq ".data.edges | length"
```

## Next steps

- Installation options: `docs/user-guide/installation.md`
- CLI usage: `docs/user-guide/cli.md`
- Discovery modes: `docs/user-guide/discovery-modes.md`
- Helm values: `docs/reference/helm.md`
- Metrics: `docs/reference/metrics.md`
