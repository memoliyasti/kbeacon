
# Installation

## Helm chart

KBeacon publishes a public GHCR image for this repository. The default install does not require an image pull Secret.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.tag=0.2.2

## Digest pinning

For production, you can pin the image by digest.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=ghcr.io/memoliyasti/kbeacon \
      --set image.digest=sha256:<digest>

## Private registry or forked image

Only use `imagePullSecrets` when you publish your own private image or deploy from a private registry.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set image.repository=<private-registry>/<namespace>/kbeacon \
      --set image.tag=<tag> \
      --set imagePullSecrets[0].name=<registry-pull-secret>

## Low-privilege install

Use this mode when the KBeacon ServiceAccount must not read Kubernetes Secret objects.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set resourcesToWatch.core.secrets=false

KBeacon will still discover workload references from Pod specs and explicit annotations, but referenced Secrets are marked as unobservable.

## Prometheus scraping

Prometheus Operator users should prefer ServiceMonitor.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set serviceMonitor.enabled=true

Clusters that use annotation-based Prometheus discovery can enable Service annotations.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set prometheus.scrapeAnnotations.enabled=true

## Local development

Use Minikube only for local development and smoke tests.

    ./hack/local-dev/deploy-incluster-minikube.sh
