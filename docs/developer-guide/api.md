# API

KBeacon exposes a read-only HTTP API.

OpenAPI contract:

    docs/api/openapi.yaml

Common endpoints:

- `/healthz`
- `/readyz`
- `/api/v1`
- `/api/v1/secrets`
- `/api/v1/workloads`
- `/api/v1/dependency-map`
- `/api/v1/secrets/{namespace}/{name}/impact`
- `/api/v1/workloads/{namespace}/{kind}/{name}/dependencies`

Example:

    curl -sS http://127.0.0.1:8080/api/v1/secrets | jq
