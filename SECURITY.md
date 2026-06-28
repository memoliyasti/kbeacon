# Security Policy

KBeacon is designed to avoid exporting Kubernetes Secret values. It does observe Secret metadata and dependency relationships, which may be sensitive in many organizations.

## Supported versions

KBeacon is pre-1.0. Security fixes target the latest released version and `main`.

| Version | Supported |
| --- | --- |
| `0.1.x` | yes |
| older | no |

## Reporting a vulnerability

Please do not report security vulnerabilities in public GitHub issues.

Use GitHub private vulnerability reporting:

https://github.com/memoliyasti/kbeacon/security/advisories/new

Include:

- affected version or commit;
- impact;
- reproduction steps;
- relevant logs with secrets redacted;
- whether Secret values, credentials, or cluster access could be exposed.

## Security principles

KBeacon must:

- never expose Secret `data` or `stringData`;
- use read-only Kubernetes RBAC;
- never mutate workloads or Secrets;
- keep the Agent API internal by default;
- treat Secret names and dependency metadata as sensitive;
- document all metrics that include Secret or workload identifiers;
- avoid logging Secret names unless explicitly configured and documented.

## Deployment guidance

Protect access to:

- the KBeacon Agent API;
- Prometheus and Mimir;
- Grafana dashboards and alert data;
- release credentials and image pull secrets.

Use private registries and image pull secrets when the image package is private.
