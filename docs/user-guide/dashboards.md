# Dashboards

KBeacon ships Grafana dashboard JSON in two locations:

- `dashboards/`
- `charts/kbeacon/dashboards/`

Enable dashboard ConfigMaps with Helm:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --set cluster.name=prod-eu-1 \
      --set dashboards.enabled=true

The default label is:

    grafana_dashboard: "1"

This works with common Grafana dashboard sidecar configurations.
