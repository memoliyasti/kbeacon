# Project Structure

```text
kbeacon/
  .github/
    ISSUE_TEMPLATE/              # Issue forms
    workflows/                   # CI, release, and Pages workflows
    CODEOWNERS                   # Default code owners
    PULL_REQUEST_TEMPLATE.md     # PR review checklist
    dependabot.yml               # Dependency update automation
  README.md
  mkdocs.yml                     # Documentation website configuration
  docs/
    index.md                     # Documentation homepage
    concepts/why-kbeacon.md       # Why the project exists and what gap it fills
    technical-design.md          # Full design document and future roadmap
    api/openapi.yaml             # Implemented Agent REST API contract
    reference/annotations.md     # Implemented annotation reference
    reference/metrics.md         # Implemented metric reference
    reference/helm.md            # Helm usage reference
    user-guide/discovery-modes.md  # How infer, explicit, hybrid, and disabled modes work
    operator-guide/security.md  # Security and low-privilege deployment guidance
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
