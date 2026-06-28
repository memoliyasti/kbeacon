# Why KBeacon?

KBeacon exists because Secret dependency information is operationally useful, but Kubernetes and Prometheus do not expose it as a ready-to-use dependency graph.

## The problem

Before rotating a database password, renewing a certificate, changing a registry credential, or deleting an old Secret, platform teams usually need to answer:

> If this Secret changes, what workloads are affected?

That question is harder than it looks because a Secret can be referenced through several different workload fields:

- `env.valueFrom.secretKeyRef`
- `envFrom.secretRef`
- `volumes.secret`
- `imagePullSecrets`
- explicit workload annotations for non-standard cases

The references are spread across namespaces, workload kinds, teams, and deployment patterns.

## Why not just use the Kubernetes API?

The Kubernetes API is the source of truth, and KBeacon uses it. But raw Kubernetes API objects are not shaped around impact analysis.

With only Kubernetes API queries, an operator usually has to:

- query many resource types;
- inspect Pod templates and Pod specs;
- normalize Deployment, StatefulSet, DaemonSet, Job, CronJob, and Pod references;
- deduplicate multiple references from the same workload to the same Secret;
- identify missing or unobservable Secrets;
- derive fan-out, team spread, namespace spread, and impact;
- repeat the work every time a workload or Secret changes.

KBeacon performs that normalization continuously and exposes the result as an API and metrics.

## Why not just use Prometheus and Grafana?

Prometheus and Grafana are still the preferred storage, dashboard, and alerting tools for KBeacon data.

The gap is discovery. Prometheus does not discover Kubernetes Secret dependency edges by itself. It can scrape metrics after something exports those edges, but it does not read Deployment specs and build a Secret-to-workload graph on its own.

KBeacon is the exporter and discovery layer. Prometheus stores the resulting time series. Grafana visualizes and alerts on them.

## What KBeacon intentionally does not do

KBeacon does not try to become a full platform.

It does not:

- store long-term history in its own database;
- run a graph database;
- provide a custom web UI;
- mutate workloads or Secrets;
- rotate Secrets;
- install CRDs;
- install an operator by default;
- export Secret values.

## When KBeacon is useful

KBeacon is useful when a team wants:

- blast-radius visibility before Secret rotation;
- dashboards showing high fan-out Secrets;
- alerts for unresolved Secret references;
- ownership and criticality context around Secret dependencies;
- a read-only Agent that fits into Prometheus and Grafana.

## When KBeacon may not be needed

KBeacon may be unnecessary if:

- the cluster is very small;
- Secret rotation is fully automated and already dependency-aware;
- all applications follow one simple configuration pattern;
- manual `kubectl` inspection is enough for the team;
- exposing Secret names as metadata is not acceptable in the environment.

## Current scope

The current release focuses on Kubernetes workload Secret references and explicit annotations. Future roadmap items, such as ExternalSecret, SecretProviderClass, Strimzi, and Confluent Connector support, are tracked separately and should not be confused with the current stable behavior.
