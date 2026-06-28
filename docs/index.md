# KBeacon

KBeacon is a Kubernetes-native Secret Dependency Intelligence Agent.

It answers one operational question:

> If this Kubernetes Secret changes, what workloads are affected?

KBeacon runs as a lightweight read-only Agent in each cluster, builds an in-memory dependency graph, exposes Prometheus metrics, and provides a small read-only HTTP API.

## What KBeacon does

- Watches Kubernetes resources with informers.
- Discovers Secret dependencies from Pod templates and annotations.
- Calculates Secret impact and fan-out.
- Exposes Prometheus metrics.
- Provides Grafana dashboard JSON.
- Ships Helm deployment artifacts.
- Avoids exporting Secret values.

## What KBeacon does not do

KBeacon does not rotate Secrets, mutate workloads, install CRDs, run admission webhooks, store data in a database, or provide a custom UI.

## Start here

- [Quickstart](getting-started.md)
- [Installation](user-guide/installation.md)
- [Metrics reference](reference/metrics.md)
- [Annotation reference](reference/annotations.md)
- [Security model](operator-guide/security.md)
