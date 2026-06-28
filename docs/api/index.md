# Agent API

The KBeacon Agent exposes a small read-only HTTP API.

The API is intended for internal cluster access, debugging, automation, and future dashboard integrations. It does not expose Kubernetes Secret values.

## Endpoints

| Endpoint | Description |
| --- | --- |
| `/healthz` | Liveness probe. |
| `/readyz` | Readiness probe and informer cache status. |
| `/metrics` | Prometheus scrape endpoint. |
| `/api/v1` | API discovery document. |
| `/api/v1/config` | Current graph count summary. |
| `/api/v1/secrets` | List observed and referenced Secrets. |
| `/api/v1/workloads` | List normalized workloads. |
| `/api/v1/dependency-map` | Current graph map with nodes and edges. |
| `/api/v1/secrets/{namespace}/{name}/impact` | Secret impact detail. |
| `/api/v1/workloads/{namespace}/{name}/dependencies` | Workload dependencies by namespace and name. |
| `/api/v1/workloads/{namespace}/{kind}/{name}/dependencies` | Workload dependencies by namespace, kind, and name. |

## Filters

`/api/v1/secrets` supports:

- `namespace`
- `ownerTeam`
- `criticality`

`/api/v1/workloads` supports:

- `namespace`
- `ownerTeam`
- `criticality`
- `workloadKind`

The current API does not implement pagination, `changedSince`, `minImpactScore`, or server-side dependency-map slicing.

## OpenAPI

The OpenAPI source is stored at:

    docs/api/openapi.yaml

## Compatibility aliases

The Agent also exposes compatibility aliases under `/api/*`, such as `/api/secrets` and `/api/dependency-map`. New integrations should use `/api/v1/*`.
