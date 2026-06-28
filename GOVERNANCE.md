# Governance

KBeacon is currently a maintainer-led open source project.

## Scope

KBeacon focuses on Kubernetes Secret dependency intelligence:

- discovery;
- dependency graph construction;
- Secret impact scoring;
- Prometheus metrics;
- read-only Agent API;
- Helm packaging;
- Grafana dashboard and alert examples.

KBeacon intentionally avoids becoming a monitoring platform, Secret manager, policy engine, or custom UI.

## Maintainer responsibilities

Maintainers are responsible for:

- project direction;
- issue triage;
- pull request review;
- releases;
- security response;
- documentation quality;
- protecting the project's scope and security model.

## Decision making

Routine changes can be merged after CI passes and maintainer review.

Design-changing work should start as an issue or proposal, especially:

- new resource extractors;
- metric name or label changes;
- API contract changes;
- Helm RBAC changes;
- release workflow changes;
- security-sensitive behavior changes.

## Compatibility

KBeacon is pre-1.0. Maintainers should still avoid unnecessary breaking changes and document any changes to:

- metrics;
- annotations;
- API responses;
- Helm values;
- dashboard assumptions.

## Releases

Releases are created from semantic version tags. See RELEASE.md.
