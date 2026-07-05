# Compatibility

KBeacon is still pre-1.0, but several surfaces are treated as public operational contracts.

## Public compatibility surfaces

KBeacon treats these as compatibility-sensitive:

- Agent API canonical `/api/v1/*` response envelopes and resource shapes;
- compatibility aliases under `/api/*`;
- Prometheus metric family names;
- Prometheus metric labels;
- Helm values documented in the Helm reference;
- dependency source type strings documented in the supported-resource matrix.

## API compatibility

The canonical API prefix is `/api/v1`.

The Agent also keeps compatibility aliases for existing clients:

| Canonical endpoint | Compatibility alias |
| --- | --- |
| `/api/v1/secrets` | `/api/secrets` |
| `/api/v1/secrets/{namespace}/{name}/impact` | `/api/secrets/{namespace}/{name}/impact` |
| `/api/v1/workloads` | `/api/workloads` |
| `/api/v1/workloads/{namespace}/{kind}/{name}/dependencies` | `/api/workloads/{namespace}/{kind}/{name}/dependencies` |
| `/api/v1/dependency-map` | `/api/dependency-map` |

New clients should use `/api/v1`. Compatibility aliases are covered by tests so existing automation does not drift silently.

## Metrics compatibility

Prometheus metric names and labels are treated as public contracts.

KBeacon can add new metric families or labels in a minor or patch release when they are bounded and documented. Removing or renaming existing metric families or labels requires explicit release notes and migration guidance.

KBeacon intentionally avoids metric labels derived from dependency source paths, Secret keys, container names, environment variable names, UIDs, and per-Pod controller identity. Detailed source paths remain available through the Agent API instead of Prometheus labels.

## Helm compatibility

Documented Helm values should remain stable within the current release line.

When a value is deprecated, docs should explain the replacement before removal. Disabled resource watchers must continue to remove matching RBAC rules when the chart owns RBAC generation.

## Pre-1.0 note

KBeacon may still make breaking changes before 1.0, but such changes should be intentional, documented in the changelog, and covered by migration notes.
