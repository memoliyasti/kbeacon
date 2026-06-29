# Dashboards

KBeacon ships Grafana dashboard JSON files in two locations:

- `dashboards/`
- `charts/kbeacon/dashboards/`

Enable dashboard ConfigMaps through Helm:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --set cluster.name=prod-eu-1 \
      --set dashboards.enabled=true

Dashboards expect KBeacon metrics in Prometheus. The Prometheus `job` label depends on your scrape integration, so prefer dashboard variables or queries based on the KBeacon `cluster` label.
