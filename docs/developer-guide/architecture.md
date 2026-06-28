# Architecture

KBeacon runs one Agent per Kubernetes cluster.

```mermaid
flowchart LR
  K8S[Kubernetes API] --> A[KBeacon Agent]
  A --> G[In-memory graph cache]
  A --> M[/metrics]
  A --> API[HTTP API]
  M --> P[Prometheus]
  P --> Grafana[Grafana]
```

## Agent components

- Kubernetes client configuration
- Shared informer factory
- resource informers
- dependency extractors
- graph cache
- Prometheus collectors
- HTTP API server

## Data flow

1. Informers observe Kubernetes resources.
2. Extractors normalize workload-to-Secret edges.
3. The graph cache calculates current impact.
4. Metrics and API responses are served from the cache.
