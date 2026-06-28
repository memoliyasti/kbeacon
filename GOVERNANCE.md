# KBeacon Governance

KBeacon is currently a maintainer-led open source project.

## Principles

- Keep the Agent lightweight.
- Use Prometheus and Grafana instead of custom storage and UI.
- Avoid exporting Secret values.
- Keep Kubernetes access read-only.
- Prefer simple, reviewable changes over large rewrites.

## Maintainers

Maintainers are responsible for:

- reviewing pull requests;
- triaging issues;
- cutting releases;
- preserving project scope;
- documenting public API and metric changes.

## Decision making

Routine changes can be merged after maintainer review and CI success.

Design-changing work should start as an issue or design discussion. Examples:

- new metric names or labels;
- API contract changes;
- new Kubernetes resource extractors;
- Helm chart security changes;
- release workflow changes.

## Release process

Releases are created from semantic version tags. See `docs/operator-guide/releases.md`.
