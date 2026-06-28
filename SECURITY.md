# Security Policy

KBeacon is designed to avoid exporting Secret values. It does observe Secret metadata and may require sensitive read-only Kubernetes RBAC.

## Supported versions

KBeacon is pre-1.0. Security fixes target the latest release line.

## Reporting a vulnerability

Please do not open a public issue for suspected vulnerabilities.

Use GitHub private vulnerability reporting if enabled. Otherwise contact the maintainer privately and include:

- affected version;
- deployment mode;
- impact summary;
- reproduction steps;
- any logs or manifests needed to understand the issue.

## Security principles

- KBeacon must not expose Secret `data` or `stringData`.
- KBeacon must use read-only Kubernetes permissions.
- KBeacon must not mutate workloads or Secrets.
- KBeacon's API should remain internal by default.
- Secret names and dependency metadata may be sensitive.
