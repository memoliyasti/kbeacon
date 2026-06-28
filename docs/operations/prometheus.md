# Prometheus

KBeacon exposes metrics at `/metrics`.

Example scrape target:

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

Prometheus Operator users can enable `serviceMonitor.enabled=true`.
