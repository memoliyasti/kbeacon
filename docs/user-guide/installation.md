# Installation

## Helm chart

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1

## Private GHCR image pull

Create a Kubernetes image pull secret from your Docker config.

    read -rs "Registry token: " REGISTRY_TOKEN
    echo

    printf '%s' "${REGISTRY_TOKEN}" | docker login ghcr.io \
      -u <github-user> \
      --password-stdin

    kubectl create namespace kbeacon-system --dry-run=client -o yaml | kubectl apply -f -

    kubectl -n kbeacon-system create secret generic ghcr-pull-secret \
      --type=kubernetes.io/dockerconfigjson \
      --from-file=.dockerconfigjson="${HOME}/.docker/config.json"

    unset REGISTRY_TOKEN

Install with the pull secret.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.1.2 \
      --set 'imagePullSecrets[0].name=ghcr-pull-secret'

## Digest pinning

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.digest=sha256:<digest>

## ServiceMonitor

Enable this only when Prometheus Operator CRDs are installed.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set serviceMonitor.enabled=true
