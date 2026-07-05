# Changelog

## Unreleased

- Added API compatibility alias tests, Prometheus metric label contract tests, and a documented compatibility policy.

## v0.3.10

- Added ReplicaSet owner-resolution cache so Deployment-managed Pods are deduplicated when their ReplicaSet owner can be resolved, while unresolved controlled Pods remain visible as Pod fallbacks.
- Added Kind E2E coverage for ReplicaSet owner-resolution runtime behavior.
- Kept ReplicaSet watching read-only and enabled by default only as an owner-resolution cache; ReplicaSets are not emitted as primary workload nodes.
- Kept existing Agent, Helm, metrics, API, and supply-chain behavior compatible with v0.3.9.

## v0.3.9

- Added optional Strimzi KafkaConnector discovery from Strimzi Kubernetes Config Provider Secret references with Helm RBAC and validation coverage.
- Added optional Confluent for Kubernetes Connector discovery from connect REST authentication Secret refs and mounted Secret file references with Helm RBAC and validation coverage.
- Added Kind E2E coverage for optional Kafka connector runtime discovery.
- Kept Kafka connector CRD watching disabled by default and opt-in through `resourcesToWatch.strimzi.kafkaConnectors` and `resourcesToWatch.confluent.connectors`.
- Kept existing Agent, Helm, metrics, API, and supply-chain behavior compatible with v0.3.8.

## v0.3.8

- Added Kind E2E coverage for optional SecretProviderClass runtime discovery.
- Added optional Secrets Store CSI Driver SecretProviderClass discovery from `spec.secretObjects[*].secretName` with Helm RBAC and validation coverage.
- Kept SecretProviderClass watching disabled by default and opt-in through `resourcesToWatch.secretsStore.secretProviderClasses`.
- Kept existing Agent, Helm, metrics, API, and supply-chain behavior compatible with v0.3.7.

## v0.3.7

- Added optional External Secrets Operator ExternalSecret discovery from target Secret metadata with Helm RBAC and validation coverage.
- Added Kind E2E coverage for optional ExternalSecret runtime discovery.
- Kept ExternalSecret watching disabled by default and opt-in through `resourcesToWatch.externalSecrets.externalSecrets`.
- Kept existing Agent, Helm, metrics, API, and supply-chain behavior compatible with v0.3.6.

## v0.3.6

- Added optional cert-manager Certificate discovery from `spec.secretName` with Helm RBAC and validation coverage.
- Added Kind E2E coverage for optional cert-manager Certificate runtime discovery.
- Kept cert-manager Certificate watching disabled by default and opt-in through `resourcesToWatch.certManager.certificates`.
- Kept existing Agent, Helm, metrics, and API behavior compatible with v0.3.5.

## v0.3.5

- Fixed `kbeaconctl snapshot export` to include the top-level `cluster` field and added stricter snapshot export validation.
- Fixed namespace-scoped installs with exactly one included namespace so the Agent uses namespace-scoped Kubernetes informers instead of cluster-wide list/watch calls.
- Added Kind E2E coverage for namespace-scoped low-privilege runtime behavior.
- Removed repository local-cluster helper artifacts and Minikube-facing product documentation; local clusters remain a validation target, not a product install path.
- Kept Agent, Helm, metrics, and API behavior compatible with v0.3.4.

## v0.3.4

- Added `kbeaconctl snapshot export` for portable Agent API snapshots.
- Added `kbeaconctl snapshot diff` with text, JSON, and markdown output for offline review and PR comments.
- Added Kind E2E smoke coverage for snapshot export and snapshot diff.
- Added Prometheus alert runbooks and runbook validation for alert rule maintenance.
- Kept Agent, Helm, metrics, and API behavior compatible with v0.3.3.

## v0.3.3

- Added `kbeaconctl` CLI foundation and Secret impact report output.
- Updated the release workflow to publish `kbeaconctl` binaries for Linux and macOS.
- Kept Agent, Helm, metrics, and API behavior compatible with v0.3.2.

## v0.3.2

- Enforced single-replica Agent mode until leader election is implemented.
- Added projected Secret volume discovery from `volumes.projected.sources.secret`.
- Added `privacy.redaction.secretKeys` to redact Secret key names in Agent API source paths.
- Added Kind end-to-end smoke testing for chart, RBAC, discovery, API, projected Secret volume discovery, and redaction behavior.
- Added release SBOM and artifact attestation wiring.
- Added a supported resource matrix for current runtime support and future resource scope.
- Added a browser-friendly Helm repository landing page.

## v0.3.1

### Changed

- Signed Helm chart packages now publish provenance files for Artifact Hub and Helm verification.
- Updated Artifact Hub chart signing metadata to use the stable public signing key URL.

### Notes

- No Agent runtime behavior changes from v0.3.0.


## v0.3.0

### Added

- Added Ingress TLS Secret discovery from networking.k8s.io/v1 Ingress resources.
- Added ServiceAccount imagePullSecrets fallback discovery for workloads that omit Pod-level `imagePullSecrets`.

### Notes

- This release adds new read-only discovery paths for ServiceAccount image pull Secret fallbacks and Ingress TLS Secret references.
- The chart now includes default read-only RBAC for ServiceAccounts and Ingresses. Disable them with `resourcesToWatch.core.serviceAccounts=false` or `resourcesToWatch.networking.ingresses=false` when those discovery paths are out of scope.

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
