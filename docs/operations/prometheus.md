
# Prometheus

KBeacon exposes Prometheus metrics at `/metrics`.

There are three supported scrape integration styles.

## Prometheus Operator ServiceMonitor

Use this when your cluster runs Prometheus Operator.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set serviceMonitor.enabled=true \
      --set serviceMonitor.labels.release=kube-prometheus-stack

## Service scrape annotations

Use this only when your Prometheus configuration discovers Services with `prometheus.io/*` annotations.

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set prometheus.scrapeAnnotations.enabled=true

The chart renders these annotations on the KBeacon Service:

    prometheus.io/scrape: "true"
    prometheus.io/path: "/metrics"
    prometheus.io/port: "8080"

## Static scrape config

Use this when scrape targets are managed centrally.

    scrape_configs:
      - job_name: kbeacon-agent
        honor_labels: true
        metrics_path: /metrics
        static_configs:
          - targets:
              - kbeacon.kbeacon-system.svc.cluster.local:8080
            labels:
              cluster: prod-eu-1
              app: kbeacon
              component: agent

## Query portability

KBeacon metrics always include the `cluster` label. The Prometheus `job` label depends on how the target is scraped, so dashboards should not require a single hard-coded job name.
