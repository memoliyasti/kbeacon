# Installation

## Helm install

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1

## Private GHCR image

If the image package is private, create an image pull secret:

    kubectl -n kbeacon-system create secret docker-registry ghcr-pull-secret \
      --docker-server=ghcr.io \
      --docker-username=<github-user> \
      --docker-password=<token-with-read-packages> \
      --docker-email=<email>

Install with:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.1.0 \
      --set 'imagePullSecrets[0].name=ghcr-pull-secret'

## Prometheus Operator

If Prometheus Operator CRDs are installed:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set serviceMonitor.enabled=true \
      --set serviceMonitor.labels.release=kube-prometheus-stack
