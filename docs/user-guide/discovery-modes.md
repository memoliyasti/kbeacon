# Discovery modes

KBeacon discovery modes control how workload-to-Secret dependencies are extracted from Kubernetes resources.

The mode can be set globally through Helm values and overridden per workload with annotations.

Discovery modes affect dependency extraction only. They do not change Kubernetes Secret type, RBAC, or whether Secret objects are readable.

## Mode summary

| Mode | Behavior | Typical use case |
| --- | --- | --- |
| `infer` | Discover dependencies only from standard Kubernetes workload specs. | Workloads that use normal Pod Secret references. |
| `explicit` | Use only KBeacon explicit dependency annotations. | Workloads where dependencies are not visible in Pod specs. |
| `hybrid` | Combine inferred and explicit dependencies. | Recommended default for most clusters. |
| `disabled` | Ignore the workload. | Exclude noisy, irrelevant, or intentionally hidden workloads. |

## Global default mode

The global default mode is configured through Helm values.

```yaml
discovery:
  defaultMode: hybrid
```

`hybrid` is the recommended default because it preserves automatic Kubernetes discovery while allowing explicit modeling for non-standard references.

## Per-workload override

Workloads can override the global default with `kbeacon.io/discovery-mode`.

```yaml
metadata:
  annotations:
    kbeacon.io/discovery-mode: explicit
```

Discovery can also be disabled for a workload with either `kbeacon.io/enabled: "false"` or `kbeacon.io/discovery-mode: disabled`.

```yaml
metadata:
  annotations:
    kbeacon.io/enabled: "false"
```

## Inferred discovery

Inferred discovery reads standard Kubernetes Secret references from workload Pod specs.

Implemented inferred sources:

| Source | Kubernetes field | Notes |
| --- | --- | --- |
| Environment variable Secret key reference | `env.valueFrom.secretKeyRef` | Records the referenced Secret object and source path. |
| Environment import from Secret | `envFrom.secretRef` | Records the referenced Secret object. |
| Secret volume | `volumes.secret` | Records the referenced Secret object and volume source. |
| Image pull Secret | `imagePullSecrets` | Controlled by `discovery.includeImagePullSecrets`. |

Container coverage is controlled by discovery configuration.

```yaml
discovery:
  includeInitContainers: true
  includeEphemeralContainers: true
  includeImagePullSecrets: true
```

When a referenced Secret key is present in a Kubernetes field, KBeacon may retain it in API source details. Metrics aggregate at Secret-object level and intentionally avoid Secret key labels.

## Explicit discovery

Explicit discovery uses KBeacon annotations instead of inferred Pod spec references.

Supported explicit dependency annotations:

- `kbeacon.io/watch-secrets`
- `kbeacon.io/watch-secrets-json`

Comma-separated form:

```yaml
metadata:
  annotations:
    kbeacon.io/watch-secrets: payments-db,shared/platform-ca,legacy-payment-token#token
```

JSON string array form:

```yaml
metadata:
  annotations:
    kbeacon.io/watch-secrets-json: |
      ["payments-db","shared/platform-ca","legacy-payment-token#token"]
```

Explicit Secret reference grammar:

```text
secret
secret#key
namespace/secret
namespace/secret#key
```

Use explicit discovery when the dependency exists outside standard Pod spec fields, for example dynamic application configuration, controller-managed references, or platform-owned operational knowledge.

## Hybrid discovery

Hybrid discovery combines inferred and explicit dependencies.

In hybrid mode:

- standard Pod spec references are discovered automatically;
- explicit annotation references are added to the same workload;
- duplicate workload-to-Secret edges are merged;
- if a Secret is found by both inferred and explicit paths, the merged edge is represented as hybrid in graph data.

Hybrid mode is appropriate for most workloads because it supports both Kubernetes-native references and explicit dependency modeling.

## Disabled mode

Disabled mode excludes a workload from dependency extraction.

```yaml
metadata:
  annotations:
    kbeacon.io/discovery-mode: disabled
```

Use disabled mode carefully. It removes the workload from dependency views and can hide real blast-radius relationships.

Common use cases:

- short-lived test workloads;
- noisy platform helper workloads;
- workloads that should not be represented in team dashboards;
- phased adoption where a namespace is enabled but selected workloads are excluded.

## Ignoring selected Secrets

`kbeacon.io/ignore-secrets` suppresses selected references after discovery.

```yaml
metadata:
  annotations:
    kbeacon.io/ignore-secrets: default-token,shared/noisy-secret
```

This applies after inferred and explicit discovery. It should be used only for intentional noise reduction.

## Namespace selection

Namespace filters apply before workload discovery.

```yaml
discovery:
  namespaces:
    include: []
    exclude:
      - kube-system
      - kube-public
      - kube-node-lease
```

Behavior:

- `include: []` means all namespaces are eligible unless excluded;
- a non-empty `include` list acts as an allow-list;
- `exclude` takes precedence over `include`.

Use namespace selection to align discovery with tenancy boundaries and platform ownership.

## Workload metadata and ownership

Discovery modes determine dependency extraction. Ownership metadata is resolved separately.

KBeacon can read ownership and classification from annotations:

```yaml
metadata:
  annotations:
    kbeacon.io/owner-team: payments-platform
    kbeacon.io/service: payments-api
    kbeacon.io/environment: prod
    kbeacon.io/criticality: critical
```

When metadata label fallback is enabled, KBeacon can also read existing Kubernetes labels.

Precedence:

1. KBeacon annotations.
2. Workload object labels.
3. Pod template labels when pod template metadata is read.

## Pod template annotations

Controller workloads can define annotations on the workload object or the Pod template.

```yaml
discovery:
  readPodTemplateAnnotations: true
```

Object-level annotations are the preferred place for workload-level KBeacon intent. Pod template annotations are useful when teams already manage metadata in the template.

## Low-privilege interaction

Discovery modes do not grant or remove Secret RBAC.

Low-privilege mode is controlled by resource watcher configuration.

```yaml
resourcesToWatch:
  core:
    secrets: false
```

When Secret object watching is disabled:

- workloads are still discovered according to their discovery mode;
- workload-to-Secret references are still extracted;
- referenced Secrets are represented as unobservable;
- dependency edges use `resolved=false`;
- Secret type and Secret change metadata are unavailable.

## Choosing a mode

Recommended decision model:

| Situation | Recommended mode |
| --- | --- |
| Standard Kubernetes workloads with Secret references in Pod specs. | `hybrid` or `infer` |
| Workloads with both Pod spec references and additional hidden dependencies. | `hybrid` |
| Workloads whose dependencies are only known through annotations or external knowledge. | `explicit` |
| Workloads that should not appear in KBeacon outputs. | `disabled` |
| Early adoption across a cluster with mixed workloads. | `hybrid` globally, selective overrides per workload |

## Example hybrid workload

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments-api
  namespace: payments
  annotations:
    kbeacon.io/discovery-mode: hybrid
    kbeacon.io/owner-team: payments-platform
    kbeacon.io/service: payments-api
    kbeacon.io/environment: prod
    kbeacon.io/criticality: critical
    kbeacon.io/watch-secrets: shared/platform-ca,legacy-payment-token
spec:
  template:
    spec:
      containers:
        - name: app
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: payments-db
                  key: password
```

This workload produces inferred dependency edges for Pod spec Secret references and explicit dependency edges for annotation references.

## Validation guidance

Recommended checks when changing discovery behavior:

- confirm workloads appear or disappear as expected in the Agent API;
- confirm inferred and explicit dependency edges are present in dependency-map responses;
- verify ignored references are intentionally absent;
- review `kbeacon_dependency_edges` labels when edge metrics are enabled;
- validate dashboard filters for namespace, owner team, criticality, and discovery mode.

## Related documentation

- Annotation reference: `docs/reference/annotations.md`
- Configuration: `docs/user-guide/configuration.md`
- Metrics reference: `docs/reference/metrics.md`
- API contract: `docs/api/openapi.yaml`
