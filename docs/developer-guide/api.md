# API

The Agent exposes a read-only HTTP API.

Important endpoints:

- `/healthz`
- `/readyz`
- `/metrics`
- `/api/v1`
- `/api/v1/secrets`
- `/api/v1/workloads`
- `/api/v1/dependency-map`
- `/api/v1/secrets/{namespace}/{name}/impact`
- `/api/v1/workloads/{namespace}/{kind}/{name}/dependencies`

The OpenAPI source is stored at `docs/api/openapi.yaml`.
