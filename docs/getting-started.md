
# Getting started

## Install with Helm

KBeacon publishes a public GHCR image for this repository. Kubernetes does not need an image pull Secret for the default install.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.2.2

## Verify the Agent

    kubectl -n kbeacon-system rollout status deploy/kbeacon
    kubectl -n kbeacon-system logs deploy/kbeacon --tail=100

Port-forward the Agent API.

    kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080

Query the Agent.

    curl -sS http://127.0.0.1:8081/readyz | jq
    curl -sS http://127.0.0.1:8081/api/v1/config | jq
    curl -sS http://127.0.0.1:8081/api/v1/secrets | jq
    curl -sS http://127.0.0.1:8081/api/v1/workloads | jq

## Prometheus scraping

KBeacon exposes metrics at `/metrics`.

Prometheus Operator users should prefer ServiceMonitor.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set serviceMonitor.enabled=true

Clusters that use Prometheus annotation discovery can enable Service annotations instead.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set prometheus.scrapeAnnotations.enabled=true

Clusters that manage scrape config centrally can scrape the Service directly.

    kbeacon.kbeacon-system.svc.cluster.local:8080

## Local Minikube workflow

Minikube is kept as a development and smoke-test workflow. It is not the production installation path.

    ./hack/local-dev/deploy-incluster-minikube.sh
    ./hack/local-dev/configure-prometheus-incluster.sh
    ./hack/local-dev/smoke-incluster.sh
