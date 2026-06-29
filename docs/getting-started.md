# Getting started

## Install with Helm

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.2.2

## Public GHCR image

If the GHCR package is public, Kubernetes does not need an image pull Secret.

## Private GHCR image

If the GHCR package is private, create a pull Secret with a classic GitHub PAT that has read:packages.

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
      --set image.tag=0.2.2 \
      --set 'imagePullSecrets[0].name=ghcr-pull-secret'

## Verify

    kubectl -n kbeacon-system rollout status deploy/kbeacon
    kubectl -n kbeacon-system logs deploy/kbeacon --tail=100

Port-forward the Agent API.

    kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080

Query the Agent.

    curl -sS http://127.0.0.1:8081/readyz | jq
    curl -sS http://127.0.0.1:8081/api/v1/config | jq
    curl -sS http://127.0.0.1:8081/api/v1/secrets | jq
    curl -sS http://127.0.0.1:8081/api/v1/workloads | jq

## Local Minikube workflow

Minikube is kept as a development and smoke-test workflow. It is not the production installation path.

    ./hack/local-dev/deploy-incluster-minikube.sh
    ./hack/local-dev/configure-prometheus-incluster.sh
    ./hack/local-dev/smoke-incluster.sh
