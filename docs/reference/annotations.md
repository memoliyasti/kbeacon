# Annotations Reference

KBeacon annotations provide explicit workload dependency modeling and ownership metadata on top of standard Kubernetes workload specs.

Annotations are optional. KBeacon can infer most Secret references from Pod specs, and it can derive ownership metadata from existing labels when metadata label fallback is enabled.

Use annotations when a workload needs an explicit override, a non-standard dependency, or metadata that should not be inferred from labels.

## Supported annotation keys

| Annotation | Applies to | Purpose |
| --- | --- | --- |
| `kbeacon.io/enabled` | Workloads | Enable or disable KBeacon discovery for the workload. |
| `kbeacon.io/discovery-mode` | Workloads | Override the default discovery mode. |
| `kbeacon.io/watch-secrets` | Workloads | Add explicit Secret dependencies with comma-separated references. |
| `kbeacon.io/watch-secrets-json` | Workloads | Add explicit Secret dependencies with a JSON string array. |
| `kbeacon.io/ignore-secrets` | Workloads | Suppress selected inferred or explicit Secret dependencies. |
| `kbeacon.io/owner-team` | Workloads and Secrets | Ownership metadata. |
| `kbeacon.io/service` | Workloads | Service or application metadata. |
| `kbeacon.io/environment` | Workloads | Environment metadata. |
| `kbeacon.io/criticality` | Workloads and Secrets | Criticality metadata. |

## Workload coverage

Workload annotations are interpreted for normalized workloads discovered by KBeacon:

- `Pod`;
- `Deployment`;
- `StatefulSet`;
- `DaemonSet`;
- `Job`;
- `CronJob`;
- `Ingress` when `resourcesToWatch.networking.ingresses=true`.

Ingress objects are modeled as Secret-consuming Kubernetes objects rather than runtime Pods. When Ingress watching is enabled, KBeacon reads Ingress metadata annotations for discovery mode, explicit dependencies, ignored dependencies, ownership metadata, service metadata, environment metadata, and criticality metadata.

For controller workloads, annotations can be placed on the workload object. Pod template annotations can also be read when `discovery.readPodTemplateAnnotations=true`.

When both object annotations and Pod template annotations are present, object-level annotations are the preferred source for workload-level intent.

## Secret metadata annotations

Secret annotations are used only as metadata. KBeacon does not read, export, or store Secret values.

```yaml
metadata:
  annotations:
    kbeacon.io/owner-team: payments-platform
    kbeacon.io/criticality: critical
```

Secret metadata can affect API responses, metric labels, dashboard grouping, and impact scoring. Secret data and stringData are never exported.

## Enable or disable discovery

`kbeacon.io/enabled` is a workload-level switch.

```yaml
metadata:
  annotations:
    kbeacon.io/enabled: "false"
```

Behavior:

| Value | Behavior |
| --- | --- |
| absent | Use the configured default discovery behavior. |
| `"true"` | Allow discovery for the workload. |
| `"false"` | Ignore the workload and emit no dependency edges for it. |

Use this annotation to suppress discovery for infrastructure helper workloads, noisy test workloads, or objects that should not appear in dependency views.

## Discovery mode override

`kbeacon.io/discovery-mode` overrides `discovery.defaultMode` for one workload.

```yaml
metadata:
  annotations:
    kbeacon.io/discovery-mode: hybrid
```

Supported values:

| Mode | Behavior |
| --- | --- |
| `infer` | Discover Secret references from Kubernetes workload specs. |
| `explicit` | Use only explicit KBeacon dependency annotations. |
| `hybrid` | Combine inferred and explicit dependencies. |
| `disabled` | Ignore the workload. |

`hybrid` is the recommended default for most workloads.

## Explicit Secret references

Use explicit dependency annotations when a Secret dependency is not visible in a standard Pod spec field.

Common examples:

- application configuration references a Secret by name;
- a controller or sidecar resolves a Secret dynamically;
- a third-party resource is represented through a workload annotation;
- dependency ownership is known by the platform team even when it is not inferable.

## Reference grammar

Explicit Secret references use this grammar:

```text
secret
secret#key
namespace/secret
namespace/secret#key
```

Rules:

- `secret` refers to a Secret in the workload namespace;
- `namespace/secret` models a cross-namespace dependency;
- `#key` is accepted for readability and compatibility, but KBeacon impact and graph aggregation are Secret-object level;
- comma-separated annotations should not include empty trailing entries.

## `kbeacon.io/watch-secrets`

Comma-separated explicit dependency list.

```yaml
metadata:
  annotations:
    kbeacon.io/watch-secrets: payments-db,shared/platform-ca,legacy-token#token
```

This annotation is easy to read and works well for short dependency lists.

## `kbeacon.io/watch-secrets-json`

JSON string array explicit dependency list.

```yaml
metadata:
  annotations:
    kbeacon.io/watch-secrets-json: |
      ["payments-db","shared/platform-ca","legacy-token#token"]
```

Use the JSON form when values are generated by automation or when comma-separated strings become difficult to manage.

The supported JSON contract is an array of string references. Object-form dependency definitions are not part of the current implemented annotation contract.

## Ignoring selected Secrets

`kbeacon.io/ignore-secrets` suppresses selected dependencies after discovery.

```yaml
metadata:
  annotations:
    kbeacon.io/ignore-secrets: default-token,shared/noisy-secret
```

Use this annotation carefully. It removes matching Secret references from the workload dependency graph and can hide real blast-radius relationships.

Typical use cases:

- suppressing known platform-managed references that are not useful for team dashboards;
- excluding test-only or generated references;
- reducing noise during phased adoption.

## Ownership metadata

Workload metadata annotations are optional but useful for grouping dashboards, alerts, and API responses.

```yaml
metadata:
  annotations:
    kbeacon.io/owner-team: payments-platform
    kbeacon.io/service: payments-api
    kbeacon.io/environment: prod
    kbeacon.io/criticality: critical
```

Recommended conventions:

| Field | Recommended value style | Example |
| --- | --- | --- |
| `owner-team` | stable platform or application team id | `payments-platform` |
| `service` | stable service or application id | `payments-api` |
| `environment` | deployment environment | `prod` |
| `criticality` | one of `unknown`, `low`, `medium`, `high`, `critical` | `critical` |

## Annotation and label precedence

KBeacon supports workload metadata label fallback through `discovery.metadataLabels`.

Precedence for ownership and classification metadata:

1. KBeacon annotations.
2. Workload object labels.
3. Pod template labels when pod template metadata is enabled.

Use annotations for explicit overrides. Use label fallback for broad adoption across workloads that already have standard ownership labels.

## Inferred dependency sources

Annotations are not required for standard Kubernetes Secret references.

KBeacon can infer dependencies from:

- `env.valueFrom.secretKeyRef`;
- `envFrom.secretRef`;
- `volumes.secret`;
- `imagePullSecrets`.

Explicit annotations are merged with inferred dependencies in `hybrid` mode.

## Example workload annotation block

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments-api
  namespace: payments
  annotations:
    kbeacon.io/enabled: "true"
    kbeacon.io/discovery-mode: hybrid
    kbeacon.io/owner-team: payments-platform
    kbeacon.io/service: payments-api
    kbeacon.io/environment: prod
    kbeacon.io/criticality: critical
    kbeacon.io/watch-secrets: shared/platform-ca,legacy-payment-token
```

## Security considerations

KBeacon annotations must not contain Secret values.

Safe annotation content:

- Secret names;
- Secret namespaces;
- ownership metadata;
- service metadata;
- environment metadata;
- criticality metadata.

Avoid placing credentials, tokens, connection strings, or raw configuration payloads in annotations. Kubernetes annotations are broadly visible to users and controllers that can read object metadata.

## Validation guidance

Recommended checks when adding or changing annotations:

- confirm the workload appears in the Agent API workload list;
- confirm expected Secret dependencies appear in dependency-map or workload dependency responses;
- confirm ignored references are intentionally absent;
- review Prometheus labels for owner team and criticality metadata;
- keep annotation values stable enough for dashboard grouping and alert routing.

## Related documentation

- Discovery modes: `docs/user-guide/discovery-modes.md`
- Configuration: `docs/user-guide/configuration.md`
- Metrics reference: `docs/reference/metrics.md`
- API contract: `docs/api/openapi.yaml`

## ServiceAccount imagePullSecrets fallback

ServiceAccount image pull Secret discovery is not annotation-driven.

When inferred discovery is enabled, KBeacon can discover `serviceAccount.imagePullSecrets` as fallback dependencies for workloads that do not define Pod-level `imagePullSecrets`.

Explicit KBeacon annotations remain useful for non-standard dependency relationships that are not visible in Pod specs or ServiceAccount metadata.

## Ingress TLS references

Ingress TLS Secret discovery does not require KBeacon annotations.

KBeacon discovers networking.k8s.io/v1 Ingress TLS references from:

```yaml
spec:
  tls:
    - secretName: app-tls
```

The dependency source type is `ingress.tls`.

Use `resourcesToWatch.networking.ingresses=false` to disable this watcher and omit Ingress RBAC.

## Projected Secret volumes

Kubernetes projected volumes can include Secret projections. KBeacon discovers these references from Pod specs and workload Pod templates.

Supported source path:

    spec.volumes[].projected.sources[].secret.name

KBeacon records these dependencies with source type `volumes.projected.sources.secret`. The dependency is namespace-local to the workload, matching Kubernetes Secret volume semantics.

## cert-manager Certificate resources

When `resourcesToWatch.certManager.certificates=true`, cert-manager `Certificate` objects can use the same KBeacon metadata annotations as workloads. The inferred dependency source type is `cert-manager.certificate.spec.secretName`.
