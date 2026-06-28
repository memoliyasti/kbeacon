# Architecture

KBeacon is a single lightweight Agent.

Main components:

- Kubernetes client configuration.
- Shared informer controller.
- Secret dependency extractors.
- In-memory graph cache.
- Prometheus collectors.
- Read-only HTTP API.

The Agent does not require CRDs, a database, a queue, or an operator.
