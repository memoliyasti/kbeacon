# Discovery modes

KBeacon supports four discovery modes: `infer`, `explicit`, `hybrid`, and `disabled`.

Discovery mode answers this question:

> How should KBeacon find the Secrets used by this workload?

It is different from Kubernetes Secret `type`. A Secret can be `Opaque`, `kubernetes.io/tls`, or `kubernetes.io/dockerconfigjson`; discovery mode controls how KBeacon finds references to that Secret.

## Recommended default

Use `hybrid` for most workloads.

`hybrid` lets KBeacon discover normal Kubernetes Secret references automatically and also lets teams add annotations for references that cannot be inferred from a Pod spec.

```yaml
metadata:
  annotations:
    kbeacon.io/discovery-mode: hybrid
```

## Mode comparison

| Mode | What KBeacon uses | Best for |
| --- | --- | --- |
| `infer` | Kubernetes Pod spec fields only | Standard Deployments, Jobs, Pods, and workloads with normal Secret references. |
| `explicit` | KBeacon annotations only | Non-standard applications, generated manifests, or dependencies hidden from Pod spec fields. |
| `hybrid` | Pod spec fields plus KBeacon annotations | Default production mode for most teams. |
| `disabled` | Nothing | Workloads that should be ignored by KBeacon. |

## `infer`

In `infer` mode, KBeacon reads Kubernetes workload specs and discovers Secret references from implemented Pod fields.

Implemented inferred sources:

- `env.valueFrom.secretKeyRef`
- `envFrom.secretRef`
- `volumes.secret`
- `imagePullSecrets`

Example:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: payments
  annotations:
    kbeacon.io/discovery-mode: infer
spec:
  template:
    spec:
      containers:
        - name: api
          image: busybox:1.36
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: payments-db
                  key: password
          envFrom:
            - secretRef:
                name: payments-config
          volumeMounts:
            - name: tls
              mountPath: /etc/tls
              readOnly: true
      volumes:
        - name: tls
          secret:
            secretName: payments-tls
      imagePullSecrets:
        - name: regcred
```

KBeacon will discover dependencies to:

- `payments-db`
- `payments-config`
- `payments-tls`
- `regcred`, when `discovery.includeImagePullSecrets=true`

## `explicit`

In `explicit` mode, KBeacon ignores inferred Pod spec Secret references and uses only explicit annotations.

Use this when the dependency exists operationally but is not visible in normal Pod spec fields.

Example:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
  namespace: payments
  annotations:
    kbeacon.io/discovery-mode: explicit
    kbeacon.io/watch-secrets: "payments-db#password,shared/platform-ca"
    kbeacon.io/owner-team: payments-platform
    kbeacon.io/criticality: high
spec:
  template:
    spec:
      containers:
        - name: worker
          image: busybox:1.36
          command: ["sh", "-c", "sleep 3600"]
```

KBeacon will report only the Secrets declared through `kbeacon.io/watch-secrets`.

## `hybrid`

In `hybrid` mode, KBeacon combines inferred and explicit dependencies.

Example:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: payments
  annotations:
    kbeacon.io/discovery-mode: hybrid
    kbeacon.io/watch-secrets: "shared/platform-ca"
    kbeacon.io/owner-team: payments-platform
    kbeacon.io/criticality: high
spec:
  template:
    spec:
      containers:
        - name: api
          image: busybox:1.36
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: payments-db
                  key: password
```

KBeacon will report:

- `payments-db`, discovered from `env.valueFrom.secretKeyRef`
- `shared/platform-ca`, discovered from annotation

If the same Secret is found from multiple places, KBeacon merges the edge deterministically instead of reporting duplicate dependency edges.

## `disabled`

Use `disabled` when a workload should not produce dependency edges.

```yaml
metadata:
  annotations:
    kbeacon.io/discovery-mode: disabled
```

This is useful for noisy infrastructure workloads, temporary debug Pods, or workloads where dependency metadata should not be emitted.

## Secret type versus discovery source

Kubernetes Secret `type` describes the Secret object. KBeacon discovery mode describes how a workload references the Secret.

| Secret usage | Common Secret type | How KBeacon sees it |
| --- | --- | --- |
| Database password | `Opaque` | `env.secretKeyRef`, `envFrom.secretRef`, `volumes.secret`, or explicit annotation |
| TLS certificate | `kubernetes.io/tls` or `Opaque` | `volumes.secret` or explicit annotation |
| Registry credential | `kubernetes.io/dockerconfigjson` | `imagePullSecrets` |
| Generated Secret from External Secrets Operator | Usually `Opaque` | The resulting Kubernetes Secret and workload reference are visible; ExternalSecret CRD mapping is future work |
| CSI SecretProviderClass material | Provider-specific | Direct SecretProviderClass support is future work |

KBeacon does not read or export Secret values. It uses Secret names, namespaces, metadata, and workload references.

## Managing `imagePullSecrets`

`imagePullSecrets` are enabled by default because registry credential rotations can affect workload rollouts.

Disable globally:

```yaml
discovery:
  includeImagePullSecrets: false
```

Or keep global inference enabled and ignore a specific Secret on a workload:

```yaml
metadata:
  annotations:
    kbeacon.io/ignore-secrets: "regcred"
```

## Explicit Secret reference grammar

`kbeacon.io/watch-secrets` accepts comma-separated values.

```text
secret
secret#key
namespace/secret
namespace/secret#key
```

Examples:

```yaml
metadata:
  annotations:
    kbeacon.io/watch-secrets: "db-credentials#password,jwt-signing-key,shared/platform-ca"
```

The current implementation of `kbeacon.io/watch-secrets-json` accepts a JSON array of strings using the same grammar:

```yaml
metadata:
  annotations:
    kbeacon.io/watch-secrets-json: '["db-credentials#password","shared/platform-ca"]'
```

## Ignore list

`kbeacon.io/ignore-secrets` removes matching dependencies after inference and explicit annotation parsing.

```yaml
metadata:
  annotations:
    kbeacon.io/ignore-secrets: "regcred,sidecar-token"
```

Use it for Secret references that are technically present but not useful as application dependency signals.

## Choosing a mode

| Scenario | Recommended mode |
| --- | --- |
| Normal application with `env`, `envFrom`, or Secret volumes | `hybrid` |
| Strictly annotation-managed dependency map | `explicit` |
| No annotations allowed and all dependencies are standard Pod fields | `infer` |
| Debug or system workload that should be ignored | `disabled` |
| Mixed standard Secret refs plus platform-owned shared Secrets | `hybrid` |

## Current limitations

The current Agent does not directly infer dependencies from:

- Ingress TLS configuration
- ExternalSecret CRDs
- SecretProviderClass resources
- Strimzi `KafkaConnector`
- Confluent `Connector`

Use explicit annotations for these cases until native support is implemented.

## Low-privilege discovery

Discovery modes work even when Secret watching is disabled:

```yaml
resourcesToWatch:
  core:
    secrets: false
```

This does not disable workload discovery. It only prevents KBeacon from observing Secret objects. Inferred and explicit dependency edges are still created, but they are reported as unresolved because Secret existence cannot be confirmed.

Use this when security policy allows workload reads but not Secret reads.

## Validation

After deploying a workload, check discovered dependencies through the Agent API:

```bash
kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080

curl -sS http://127.0.0.1:8081/api/v1/workloads | jq
curl -sS http://127.0.0.1:8081/api/v1/secrets | jq
curl -sS http://127.0.0.1:8081/api/v1/dependency-map | jq
```

Or query Prometheus:

```promql
kbeacon_dependency_edges{workload_namespace="payments"}
```
