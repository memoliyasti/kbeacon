# Changelog

## v0.2.0

This release focuses on making KBeacon safer, clearer, and easier to operate as a personal open source project.

### Added

- Low-privilege mode documentation and regression coverage.
- Helm rendering checks for low-privilege mode.
- Edge metric cardinality guard through `metrics.edge.enabled`.
- Server/API response tests.
- Discovery modes guide.
- Project positioning guide explaining why KBeacon exists and what it does not replace.
- More explicit roadmap boundaries.
- GitHub Pages documentation refinements.

### Changed

- Helm values and generated Agent config now better reflect implemented behavior.
- OpenAPI and API examples are aligned with implemented responses.
- README and website now position KBeacon as a Secret dependency discovery layer, not a full monitoring platform.
- Technical design is marked as both current implementation notes and future design intent.

### Security

- Documented low-privilege operation without Secret object reads.
- Reinforced that KBeacon never exports Secret values.
- Documented that Secret names and dependency metadata may still be sensitive.

## v0.1.2

Initial public release line with Helm chart, read-only Agent API, Prometheus metrics, Grafana dashboard examples, and GHCR images.
