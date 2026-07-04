# KBeacon Roadmap

KBeacon is an early-stage personal open source project. The roadmap is intentionally practical and focused on the current project goal: Secret dependency discovery for Kubernetes workloads.

## Current release line

Implemented today:

- Kubernetes workload Secret dependency discovery for Pods, workload controllers, ServiceAccounts, and Ingress TLS references.
- Secret reference discovery from environment variables, envFrom, Secret volumes, projected Secret volumes, imagePullSecrets, and KBeacon annotations.
- `infer`, `explicit`, `hybrid`, and `disabled` discovery modes.
- Read-only Agent API with filtering and bounded pagination.
- `kbeaconctl` API client, Secret impact report, snapshot export, and snapshot diff with text, JSON, and markdown output.
- Prometheus metrics, recording rules, alert rules, and operator runbooks.
- Grafana dashboard examples.
- Helm chart with single-replica Agent mode, namespace-scoped RBAC option, low-privilege mode, ServiceMonitor support, and public chart repository index.
- Secret key redaction controls for dependency source paths.
- Kind E2E smoke coverage for chart/API/CLI behavior.
- SBOMs, checksums, signed Helm chart provenance, GitHub artifact attestations, and GHCR release images.
- Supported-resource matrix and release documentation.

## Near-term priorities

### Documentation and usability

- Keep README, website, Helm reference, OpenAPI, and examples aligned with real behavior.
- Add more end-to-end examples for common production Secret patterns.

### Safety and operability

- Keep release, security, checksum, provenance, and attestation documentation accurate as the project evolves.
- Continue tightening least-privilege RBAC examples.
- Add more tests for low-privilege and namespace-scoped installs.
- Improve readiness and troubleshooting guidance.
- Keep high-cardinality metrics optional and documented.

### API and metrics maturity

- Keep API response tests aligned with OpenAPI.
- Treat metric names and labels as public contract.
- Avoid adding unbounded labels.
- Add compatibility notes before changing API or metric behavior.

## Later ideas

These are possible future features, not current promises:

- ExternalSecret target Secret modeling.
- SecretProviderClass and CSI Secret Store support.
- Strimzi KafkaConnector support.
- Confluent Connector support.
- Grafana App Plugin.
- Historical dependency timeline using Prometheus or Mimir data.
- Optional operator mode for larger fleets.

## Out of scope for now

KBeacon does not plan to become:

- a Secret rotation system;
- a policy enforcement engine;
- a security scanner;
- a custom dashboard platform;
- a graph database service;
- a replacement for Prometheus, Grafana, or Mimir.
