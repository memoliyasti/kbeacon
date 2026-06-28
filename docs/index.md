# KBeacon

KBeacon is a Kubernetes-native Secret Dependency Intelligence agent.

It answers one operational question:

> If this Secret changes, what workloads are affected?

KBeacon watches Kubernetes resources, builds an in-memory dependency graph, and exposes dependency intelligence through Prometheus metrics and a read-only HTTP API.

## Why KBeacon?

Secret rotations, certificate renewals, registry credential updates, and database credential changes can affect many workloads. KBeacon helps platform and SRE teams understand blast radius before changes become incidents.

## What KBeacon does

- Discovers Secret dependencies from Kubernetes workloads.
- Supports inferred, explicit, and hybrid discovery modes.
- Calculates Secret impact and workload fan-out.
- Exposes Prometheus metrics.
- Exposes a read-only Agent API.
- Ships Grafana dashboards and alert examples.
- Deploys as a lightweight Helm chart.

## What KBeacon does not do

- It does not export Secret values.
- It does not mutate Kubernetes resources.
- It does not install CRDs by default.
- It does not run a graph database.
- It does not replace Prometheus or Grafana.

## Quick links

- [Getting started](getting-started.md)
- [Helm reference](reference/helm.md)
- [Metrics reference](reference/metrics.md)
- [Annotations reference](reference/annotations.md)
- [Technical design](technical-design.md)
- [Contributing](community/contributing.md)
