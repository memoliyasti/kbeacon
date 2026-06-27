# Security Policy

KBeacon is designed to avoid exporting Secret values. It does, however, observe Secret metadata and may require Kubernetes RBAC that is sensitive in many organizations.

## Supported versions

This scaffold is pre-1.0. Security fixes should target the latest minor version until a stable release policy is published.

## Reporting a vulnerability

Please report suspected vulnerabilities privately to the maintainers before public disclosure.

## Security principles

- KBeacon must not expose Secret `data` or `stringData` in metrics, logs, or API responses.
- KBeacon must use read-only Kubernetes permissions.
- KBeacon must not mutate workloads or Secrets.
- KBeacon's API should remain internal by default.
- Secret names and dependency metadata may be sensitive; protect access to Prometheus, Mimir, Grafana, and the Agent API accordingly.
