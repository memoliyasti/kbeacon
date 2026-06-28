# Getting started

This guide runs KBeacon locally on Minikube in in-cluster mode.

## Prerequisites

- Kubernetes cluster, such as Minikube
- kubectl
- Helm
- Docker
- jq

## Create a demo workload

    kubectl create namespace kbeacon-demo --dry-run=client -o yaml | kubectl apply -f -

    kubectl -n kbeacon-demo create secret generic app-db-secret \
      --from-literal=username=demo \
      --from-literal=password=demo \
      --dry-run=client -o yaml | kubectl apply -f -

Apply a workload that references the Secret through env vars, envFrom, and a volume. See `README.md` for the full manifest.

## Deploy KBeacon

    ./hack/local-dev/deploy-incluster-minikube.sh

## Configure Prometheus

    ./hack/local-dev/configure-prometheus-incluster.sh

Port-forward Prometheus:

    kubectl -n kube-addons port-forward svc/prometheus-server 9090:80

Run smoke tests:

    ./hack/local-dev/smoke-incluster.sh

## Query the Agent API

    kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080

    curl -sS http://127.0.0.1:8081/readyz | jq
    curl -sS http://127.0.0.1:8081/api/v1/secrets | jq
    curl -sS http://127.0.0.1:8081/api/v1/workloads | jq
