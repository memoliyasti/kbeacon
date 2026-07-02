# Security Policy

KBeacon helps operators understand Kubernetes Secret dependency blast radius. It must be handled as security-sensitive software because Secret names, namespace names, workload names, and dependency relationships can reveal operational details.

## Supported versions

KBeacon is pre-1.0. Security fixes are provided for the latest released version.

| Version | Supported |
| --- | --- |
| Latest release | yes |
| Older pre-1.0 releases | best effort |

## Reporting a vulnerability

Do not open a public issue for suspected vulnerabilities.

Use GitHub private vulnerability reporting if enabled. Otherwise contact the maintainer privately.

Include:

- affected KBeacon version or commit;
- deployment mode;
- Kubernetes version if relevant;
- impact summary;
- reproduction steps;
- logs or manifests required to reproduce;
- whether Secret values, tokens, or private metadata may have been exposed.

Do not include real Secret values in the report unless a maintainer explicitly requests a secure transfer method.

## Security model

KBeacon:

- uses read-only Kubernetes access;
- does not mutate workloads or Secrets;
- does not export Secret data or stringData;
- stores dependency state in memory;
- exposes metrics and a read-only API;
- relies on Kubernetes RBAC and the surrounding observability stack for access control around exported metadata.

## Sensitive metadata

KBeacon does expose metadata such as Secret names, namespaces, workload names, owner teams, and dependency edges. Treat this as sensitive operational metadata.

Protect access to:

- the Agent API;
- Prometheus and Mimir;
- Grafana dashboards;
- Agent logs;
- release and deployment automation;
- image pull credentials.

## Recommended hardening

- Keep the Agent Service internal.
- Use NetworkPolicy where possible.
- Restrict access to Prometheus, Mimir, and Grafana.
- Use read-only RBAC.
- Prefer namespace filters when full-cluster visibility is not needed.
- Use private GHCR packages only when necessary.
- Rotate image pull tokens regularly.
- Prefer short-lived or narrowly scoped credentials.
- Do not paste PATs, kubeconfigs, Secret values, or production identifiers into public issues.

## Dependency and supply-chain security

The project uses Go modules, Docker images, Helm charts, GitHub Actions, GitHub Pages, and GHCR. Dependency updates are grouped through Dependabot. Security-sensitive dependency updates should be reviewed and released promptly.

## Non-goals

KBeacon is not a Secret scanner, Secret manager, vulnerability scanner, or policy engine. It does not validate Secret strength, encryption, rotation correctness, or compliance status.

## Automated security checks

The repository runs automated security checks for:

- Go vulnerability reachability with `govulncheck`;
- secret leak detection with Gitleaks;
- filesystem vulnerability and misconfiguration scanning with Trivy.

These checks are intended to complement, not replace, code review and dependency review. Secret names and operational metadata can still be sensitive even when Secret values are not exposed.
