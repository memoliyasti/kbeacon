# KBeacon Annotation Reference

KBeacon annotations are optional metadata controls for discovery, ownership, and risk classification.

All annotations use the `kbeacon.io/` prefix.

## Implemented annotations

| Annotation | Scope | Values | Default | Description |
| --- | --- | --- | --- | --- |
| `kbeacon.io/enabled` | Workload | `true`, `false` | `true` | Enables or disables discovery for a workload. |
| `kbeacon.io/discovery-mode` | Workload | `infer`, `explicit`, `hybrid`, `disabled` | Agent default | Selects discovery behavior. |
| `kbeacon.io/watch-secrets` | Workload | CSV Secret refs | empty | Explicit Secret dependency list. |
| `kbeacon.io/watch-secrets-json` | Workload | JSON array of Secret ref strings | empty | Structured explicit dependencies. |
| `kbeacon.io/ignore-secrets` | Workload | CSV Secret refs | empty | Removes matching inferred or explicit dependencies. |
| `kbeacon.io/owner-team` | Workload, Secret | team slug | empty | Ownership metadata. |
| `kbeacon.io/service` | Workload | service slug | empty | Service/application grouping. |
| `kbeacon.io/environment` | Workload | env slug | empty | Environment metadata. |
| `kbeacon.io/criticality` | Workload, Secret | `low`, `medium`, `high`, `critical` | `unknown` | Operational criticality. |

## Discovery modes

For operational examples and mode selection guidance, see [Discovery modes](../user-guide/discovery-modes.md).


### `infer`

KBeacon discovers dependencies from Pod specs:

- `env.valueFrom.secretKeyRef`
- `envFrom.secretRef`
- `volumes.secret`
- `imagePullSecrets`

### `explicit`

KBeacon uses only explicit annotations:

- `kbeacon.io/watch-secrets`
- `kbeacon.io/watch-secrets-json`

### `hybrid`

KBeacon combines inferred and explicit dependencies with deterministic deduplication.

### `disabled`

KBeacon returns no dependency edges for the annotated workload.

## `kbeacon.io/watch-secrets` grammar

```text
secret
secret#key
namespace/secret
namespace/secret#key
```

Example:

```yaml
metadata:
  annotations:
    kbeacon.io/discovery-mode: hybrid
    kbeacon.io/watch-secrets: "db-credentials#password,jwt-signing-key,shared/platform-ca"
    kbeacon.io/owner-team: payments-platform
    kbeacon.io/criticality: critical
```

## `kbeacon.io/watch-secrets-json`

The current implementation accepts a JSON array of strings using the same grammar as `kbeacon.io/watch-secrets`.

```yaml
metadata:
  annotations:
    kbeacon.io/watch-secrets-json: '["db-credentials#password","shared/platform-ca"]'
```

Object-form JSON is planned but not implemented yet.

## `kbeacon.io/ignore-secrets`

Use this when inference discovers a Secret that should not be treated as an application dependency.

```yaml
metadata:
  annotations:
    kbeacon.io/ignore-secrets: "image-pull-secret,sidecar-token"
```

## Existing label fallback

KBeacon metadata annotations are optional.

For workload ownership and classification, KBeacon first checks `kbeacon.io/*` annotations. If those annotations are absent, it can read existing workload labels configured through `discovery.metadataLabels`.

Default label keys include common platform conventions such as:

- `app.kubernetes.io/team`
- `team`
- `technical-owner`
- `business-owner`
- `app.kubernetes.io/name`
- `service`
- `app.kubernetes.io/environment`
- `environment`
- `priority`
- `tier`
- `slo-tier`

This lets teams adopt KBeacon without rolling out metadata-only annotation changes to application Pods.

## Ownership and criticality

KBeacon derives Secret metadata as follows:

1. Direct Secret annotation when present.
2. Affected workload metadata when a Secret has exactly one owner team.
3. Maximum criticality from affected workloads.
4. Built-in fallback: `unknown`.

Example:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: kbeacon-demo
  annotations:
    kbeacon.io/owner-team: platform
    kbeacon.io/criticality: high
```

If `Deployment/api` depends on `Secret/app-db-secret`, the Secret impact response can inherit:

```json
{
  "ownerTeam": "platform",
  "criticality": "high"
}
```

## Not implemented yet

These annotations are reserved in design material but are not currently interpreted by the Agent:

- namespace-level annotation inheritance;
- `kbeacon.io/include-image-pull-secrets`;
- `kbeacon.io/dependency-purpose`;
- `kbeacon.io/external-id`;
- `kbeacon.io/change-risk`;
- `kbeacon.io/notes`;
- object-form `kbeacon.io/watch-secrets-json`.

Use `docs/technical-design.md` for long-term design intent.
