# KBeacon Governance

KBeacon is currently a maintainer-led open source project.

## Roles

### Users

Users install and operate KBeacon, report bugs, request features, and provide feedback.

### Contributors

Contributors submit issues, documentation, tests, code, dashboards, alert rules, or examples.

### Maintainers

Maintainers review contributions, manage releases, triage issues, enforce project standards, and protect the security of the project.

Maintainers are listed in `MAINTAINERS.md`.

## Decision making

KBeacon uses lazy consensus for most decisions:

1. A proposal is opened as an issue or pull request.
2. Maintainers and contributors discuss trade-offs.
3. If no blocking concern remains, a maintainer may merge.

Blocking concerns should be specific, actionable, and tied to project goals such as security, reliability, maintainability, API compatibility, or metric cardinality.

## Project values

KBeacon optimizes for:

- read-only Kubernetes operation;
- no Secret value exposure;
- small operational footprint;
- Prometheus-compatible metrics;
- Grafana-compatible user workflows;
- clear documentation and contributor experience;
- stable public contracts.

## Becoming a maintainer

A contributor may be invited to become a maintainer after sustained, high-quality contributions and demonstrated care for project values.

## Code of Conduct

All participants must follow `CODE_OF_CONDUCT.md`.
