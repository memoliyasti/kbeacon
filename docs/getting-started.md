# Getting started

## Install with Helm

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1

## Install from a private GHCR package

If the package is private, create a Kubernetes image pull secret from your local Docker login.

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

Install KBeacon with the pull secret.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.1.2 \
      --set 'imagePullSecrets[0].name=ghcr-pull-secret'

## Verify the Agent

    kubectl -n kbeacon-system rollout status deploy/kbeacon
    kubectl -n kbeacon-system logs deploy/kbeacon --tail=100

Port-forward the Agent API.

    kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080

Query the Agent.

    curl -sS http://127.0.0.1:8081/readyz | jq
    curl -sS http://127.0.0.1:8081/api/v1/config | jq
    curl -sS http://127.0.0.1:8081/api/v1/secrets | jq

## Local Minikube workflow

    ./hack/local-dev/deploy-incluster-minikube.sh
    ./hack/local-dev/configure-prometheus-incluster.sh
    ./hack/local-dev/smoke-incluster.sh
