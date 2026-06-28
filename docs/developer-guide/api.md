# API

The Agent exposes a read-only HTTP API.

## Contract source

OpenAPI source:

    docs/api/openapi.yaml

Example API payloads:

    examples/api/

## Primary endpoints

- `/healthz`
- `/readyz`
- `/metrics`
- `/api/v1`
- `/api/v1/config`
- `/api/v1/secrets`
- `/api/v1/workloads`
- `/api/v1/dependency-map`
- `/api/v1/secrets/{namespace}/{name}/impact`
- `/api/v1/workloads/{namespace}/{name}/dependencies`
- `/api/v1/workloads/{namespace}/{kind}/{name}/dependencies`

## Response envelope

Most API responses use this envelope:

    {
      "apiVersion": "kbeacon.io/v1",
      "cluster": "prod-eu-1",
      "generatedAt": "2026-06-28T12:00:00Z",
      "data": {}
    }

Errors use:

    {
      "apiVersion": "kbeacon.io/v1",
      "error": {
        "code": "not_found",
        "message": "Workload not found in dependency graph"
      }
    }

## Implementation notes

- The API is backed by the in-memory graph cache.
- It is read-only.
- It does not expose Kubernetes Secret values.
- Secret names and dependency metadata may still be sensitive.
- Compatibility aliases under `/api/*` exist for older clients, but new clients should use `/api/v1/*`.
