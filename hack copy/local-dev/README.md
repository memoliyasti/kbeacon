# KBeacon local development helpers

This directory contains helper files for running KBeacon on Minikube.

## In-cluster workflow

Build the local image into the Minikube Docker daemon and install the Helm chart:

    ./hack/local-dev/deploy-incluster-minikube.sh

Configure the local Prometheus chart to scrape the in-cluster KBeacon Service:

    ./hack/local-dev/configure-prometheus-incluster.sh

Port-forward Prometheus if needed:

    kubectl -n kube-addons port-forward svc/prometheus-server 9090:80

Run smoke tests:

    ./hack/local-dev/smoke-incluster.sh

## Local-process workflow

For debugging the Agent directly on the host:

    go build -o ./bin/kbeacon-agent ./cmd/kbeacon-agent

    KBEACON_CLUSTER_NAME=minikube ./bin/kbeacon-agent \
      --http-bind-address=0.0.0.0:8080 \
      --log-level=debug

Expose the host process to Minikube through a Service and Endpoints object:

    ./hack/local-dev/create-kbeacon-local-endpoint.sh

The preferred workflow is in-cluster mode because it exercises Helm, RBAC, Service networking, Prometheus scraping and Grafana dashboards.
