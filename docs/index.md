# KBeacon

KBeacon is a lightweight Kubernetes-native Secret dependency intelligence agent.

It answers one operational question:

> If this Secret changes, what workloads are affected?

KBeacon watches Kubernetes resources with read-only access, builds an in-memory dependency graph, and exposes dependency intelligence through Prometheus metrics and a read-only Agent API.

## Why KBeacon?

Secret rotations, certificate renewals, registry credential updates, and database credential changes can affect many workloads. KBeacon helps platform and SRE teams understand blast radius before changes become incidents.


## Why not just Kubernetes API, Prometheus, and Grafana?

KBeacon does not replace those tools. It connects them.

The Kubernetes API has the raw objects, but not a normalized Secret dependency graph. Prometheus and Grafana can store, query, alert, and visualize dependency data, but they do not discover workload-to-Secret edges by themselves.

KBeacon fills that gap by watching Kubernetes resources, normalizing Secret references, and exporting the result.

Read more in [Why KBeacon?](concepts/why-kbeacon.md).

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
- Blast-radius demo: user-guide/blast-radius-demo.md
- Installation: user-guide/installation.md
- CLI: user-guide/cli.md
- Helm reference: reference/helm.md
- Supported resources: reference/supported-resources.md
- Metrics reference: reference/metrics.md
- Annotations reference: reference/annotations.md
- Security: community/security.md

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=memoliyasti/kbeacon&type=Date)](https://star-history.com/#memoliyasti/kbeacon&Date)
