
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

List endpoints and `/api/v1/dependency-map` also include `pagination`:

    {
      "limit": 100,
      "offset": 0,
      "total": 250,
      "returned": 100,
      "nextOffset": 100
    }

Errors use:

    {
      "apiVersion": "kbeacon.io/v1",
      "error": {
        "code": "not_found",
        "message": "Workload not found in dependency graph"
      }
    }

Invalid query values use `400 invalid_query`.

## Filtering and pagination

`/api/v1/secrets` supports `namespace`, `ownerTeam`, `criticality`, `exists`, `secretName`, `limit`, and `offset`.

`/api/v1/workloads` supports `namespace`, `ownerTeam`, `criticality`, `workloadKind`, `workloadName`, `discoveryMode`, `limit`, and `offset`.

`/api/v1/dependency-map` supports edge filtering with `namespace`, `workloadNamespace`, `secretNamespace`, `workloadKind`, `workloadName`, `secretName`, `ownerTeam`, `criticality`, `resolved`, `discoveryMode`, `limit`, and `offset`.

`limit` defaults to `100` and is capped at `1000`.

## Implementation notes

- The API is backed by the in-memory graph cache.
- It is read-only.
- It does not expose Kubernetes Secret values.
- Secret names and dependency metadata may still be sensitive.
- Compatibility aliases under `/api/*` exist for older clients, but new clients should use `/api/v1/*`.
