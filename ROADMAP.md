# KBeacon Roadmap

KBeacon is an early-stage personal open source project. The roadmap is intentionally practical and focused on the current project goal: Secret dependency discovery for Kubernetes workloads.

## Current release line

Implemented today:
- Ingress TLS Secret discovery for networking.k8s.io/v1 Ingress TLS Secret references.

- Kubernetes workload Secret dependency discovery.
- `infer`, `explicit`, `hybrid`, and `disabled` discovery modes.
- Read-only Agent API.
- Prometheus metrics.
- Grafana dashboard examples.
- Helm chart.
- Low-privilege mode without Secret object reads.
- Edge metric cardinality guard.
- API filtering and bounded pagination.
- Metadata fallback from existing workload labels.
- Live scale benchmark report harness.
- GitHub Actions CI, release workflow, and documentation website.

## Near-term priorities

### Documentation and usability

- Keep README, website, Helm reference, OpenAPI, and examples aligned with real behavior.
- Add more end-to-end examples for common Secret patterns.
- Keep Minikube as a local development workflow, not as the production install path.

### Safety and operability

- Release and security hardening: SBOM, provenance/attestation documentation, checksum verification, branch protection recommendations, and token hygiene.

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
