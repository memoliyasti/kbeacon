# KBeacon

KBeacon is a lightweight Kubernetes-native Secret dependency intelligence agent.

It answers one operational question:

> If this Secret changes, what workloads are affected?

KBeacon watches Kubernetes resources with read-only access, builds an in-memory dependency graph, and exposes dependency intelligence through Prometheus metrics and a read-only Agent API.

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
- It does not install KBeacon CRDs.
- It does not run a graph database.
- It does not replace Prometheus or Grafana.
- It is not a Secret scanner or Secret manager.

## First steps

- Getting started: getting-started.md
- Installation: user-guide/installation.md
- Helm reference: reference/helm.md
- Metrics reference: reference/metrics.md
- Annotations reference: reference/annotations.md
- Security: community/security.md
