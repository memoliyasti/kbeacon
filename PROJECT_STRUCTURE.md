# Project Structure

```text
kbeacon/
  README.md
  docs/
    technical-design.md          # Full production-grade design document
    api/openapi.yaml             # Agent REST API contract
    reference/annotations.md     # Implemented annotation reference
    reference/metrics.md         # Implemented metric reference
    reference/helm.md            # Helm usage reference
  charts/kbeacon/                # Helm chart for one lightweight Agent Deployment
  charts/kbeacon/dashboards/     # Dashboard ConfigMaps rendered by Helm
  dashboards/                    # Grafana dashboard JSON examples
  examples/
    annotations/                 # Workload annotation examples
    prometheus/                  # Scrape, remote_write, recording and alert rules
    api/                         # Example API responses
  cmd/kbeacon-agent/             # KBeacon Agent entrypoint
  internal/
    config/                      # Runtime config loading and defaults
    controller/                  # Kubernetes informer controller
    discovery/                   # Secret dependency extractors
    graph/                       # In-memory dependency graph cache
    kube/                        # Kubernetes client config
    metrics/                     # Prometheus collectors and recorders
    server/                      # HTTP API and health endpoints
  hack/local-dev/                # Minikube development helpers
```
