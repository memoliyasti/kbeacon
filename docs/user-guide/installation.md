# Installation

## Helm chart

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.1.2

## Public registry image

If the GHCR package is public, no Kubernetes image pull Secret is required.

## Private GHCR image pull

Create a Kubernetes image pull Secret with a classic GitHub PAT that has read:packages.

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

Install with the pull Secret.

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

## Local development

Use Minikube only for local development and smoke tests.

    ./hack/local-dev/deploy-incluster-minikube.sh
