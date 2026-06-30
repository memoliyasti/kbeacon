
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

## Pagination

List endpoints and `/api/v1/dependency-map` support bounded pagination.

| Query parameter | Default | Maximum | Description |
| --- | ---: | ---: | --- |
| `limit` | `100` | `1000` | Maximum items or edges returned. Values above `1000` are capped to `1000`. |
| `offset` | `0` | none | Zero-based result offset. Negative values are rejected. |

Paginated responses include a top-level `pagination` object. When no more results are available, `nextOffset` is omitted.

## Filters

`/api/v1/secrets` supports exact filters: `namespace`, `ownerTeam`, `criticality`, `exists`, and `secretName`.

Example:

    curl -sS "http://127.0.0.1:8081/api/v1/secrets?namespace=payments&exists=true&limit=50" | jq

`/api/v1/workloads` supports exact filters: `namespace`, `ownerTeam`, `criticality`, `workloadKind`, `workloadName`, and `discoveryMode`.

`workloadKind` and `discoveryMode` are case-insensitive.

Example:

    curl -sS "http://127.0.0.1:8081/api/v1/workloads?namespace=payments&workloadKind=Deployment" | jq

`/api/v1/dependency-map` supports edge filters: `namespace`, `workloadNamespace`, `secretNamespace`, `workloadKind`, `workloadName`, `secretName`, `ownerTeam`, `criticality`, `resolved`, and `discoveryMode`.

For dependency-map responses, pagination is applied to filtered edges. The returned `nodes` array contains only nodes connected to the returned page of edges.

Example:

    curl -sS "http://127.0.0.1:8081/api/v1/dependency-map?secretName=payments-db&resolved=true&limit=100" | jq

## Error behavior

Invalid query values return `400` with `invalid_query`.

## OpenAPI

The OpenAPI source is stored at:

    docs/api/openapi.yaml

## Compatibility aliases

The Agent also exposes compatibility aliases under `/api/*`, such as `/api/secrets` and `/api/dependency-map`. New integrations should use `/api/v1/*`.
