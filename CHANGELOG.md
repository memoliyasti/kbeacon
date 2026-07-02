# Changelog

## v0.2.4

### Changed

- Switched project license metadata to MIT.
- Refreshed README, Helm chart README, installation, configuration, Prometheus, dashboard, annotation, metric, and discovery documentation.
- Added professional inline Helm values documentation.
- Updated chart metadata and release references for the v0.2.4 patch release.

### Notes

- Agent runtime behavior is compatible with v0.2.3.
## v0.2.3

- Added API filtering and bounded pagination controls.
- Added workload metadata label fallback.
- Added Prometheus scrape annotations and preserved KBeacon metric labels in ServiceMonitor.
- Added scale benchmark reporting and expected dependency edge reporting.
- Added Grafana Node Graph dependency panels and the standalone Dependency Graph Explorer dashboard.
- Kept Agent behavior compatible with the v0.2 release line.



## v0.2.2

- Added deterministic scale fixture generation and validation targets.
- Added live demo metrics validation for the blast-radius demo.
- Added dashboard JSON validation and a PromQL dashboard query guide.
- Wired reusable quality gates into local validation and CI.
- Kept Agent behavior compatible with v0.2.1.


## v0.2.1

Documentation and release polish patch.

- Added a Secret blast-radius demo with realistic multi-namespace workloads.
- Recorded verified demo output from a live KBeacon Agent API run.
- Fixed GitHub Pages Mermaid rendering for technical design diagrams.
- Added Star History links to the README and documentation site.
- Clarified project positioning, roadmap boundaries, and implementation scope.
- Kept Agent behavior compatible with v0.2.0.

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
