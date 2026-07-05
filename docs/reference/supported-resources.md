# Supported resources

This page is the implemented support matrix for the current KBeacon release line.

The technical design and roadmap may mention future resources. The table below is the runtime contract users should rely on today.

## Current runtime support

| Kubernetes object | API group / version | Helm watcher value | Status | Dependency sources | Notes |
| --- | --- | --- | --- | --- | --- |
| Secret | core/v1 | `resourcesToWatch.core.secrets` | Supported, optional | Secret metadata, type, annotations, existence, change timestamps | KBeacon does not export Secret `data` or `stringData`. Kubernetes RBAC does not separate Secret metadata from Secret values, so this permission is sensitive. |
| Pod | core/v1 | `resourcesToWatch.core.pods` | Supported | `env.valueFrom.secretKeyRef`, `envFrom.secretRef`, `volumes.secret`, `volumes.projected.sources.secret`, `imagePullSecrets`, KBeacon annotations, metadata labels | Pod Secret references are namespace-local except explicit KBeacon annotations can name another namespace. |
| ServiceAccount | core/v1 | `resourcesToWatch.core.serviceAccounts` | Supported, optional | ServiceAccount `imagePullSecrets` fallback | Used when workload Pods omit Pod-level `imagePullSecrets`. |
| Deployment | apps/v1 | `resourcesToWatch.apps.deployments` | Supported | Pod template Secret references, KBeacon annotations, metadata labels | Rendered as one normalized workload node. |
| StatefulSet | apps/v1 | `resourcesToWatch.apps.statefulSets` | Supported | Pod template Secret references, KBeacon annotations, metadata labels | Rendered as one normalized workload node. |
| DaemonSet | apps/v1 | `resourcesToWatch.apps.daemonSets` | Supported | Pod template Secret references, KBeacon annotations, metadata labels | Rendered as one normalized workload node, not one node per Kubernetes node. |
| Job | batch/v1 | `resourcesToWatch.batch.jobs` | Supported | Pod template Secret references, KBeacon annotations, metadata labels | Rendered as one normalized workload node. |
| CronJob | batch/v1 | `resourcesToWatch.batch.cronJobs` | Supported | Job template Secret references, KBeacon annotations, metadata labels | Rendered as one normalized workload node. |
| Ingress | networking.k8s.io/v1 | `resourcesToWatch.networking.ingresses` | Supported, optional | `spec.tls[].secretName`, KBeacon annotations, metadata labels | Modeled as a Secret-consuming Kubernetes object in KBeacon outputs. |
| cert-manager Certificate | cert-manager.io/v1 | `resourcesToWatch.certManager.certificates` | Supported, optional | `spec.secretName` target Secret | Requires cert-manager CRDs to be installed before enabling the watcher. |
| ExternalSecret | external-secrets.io/v1 | `resourcesToWatch.externalSecrets.externalSecrets` | Supported, optional | `spec.target.name` target Secret, or `metadata.name` fallback when target name is omitted | Requires External Secrets Operator CRDs to be installed before enabling the watcher. |
| SecretProviderClass | secrets-store.csi.x-k8s.io/v1 | `resourcesToWatch.secretsStore.secretProviderClasses` | Supported, optional | `spec.secretObjects[*].secretName` synced Kubernetes Secret outputs | Requires Secrets Store CSI Driver CRDs to be installed before enabling the watcher. KBeacon does not inspect external provider object names or values. |
| Strimzi KafkaConnector | kafka.strimzi.io/v1 | `resourcesToWatch.strimzi.kafkaConnectors` | Supported, optional | Strimzi Kubernetes Config Provider Secret references in string values under `spec.config`, KBeacon annotations | Requires Strimzi KafkaConnector CRDs to be installed before enabling the watcher. KBeacon parses only Kubernetes Secret reference tokens such as `${secrets:namespace/name:key}` and `${secrets:name:key}`; it does not call Kafka Connect or inspect Secret values. |
| Confluent Connector | platform.confluent.io/v1beta1 | `resourcesToWatch.confluent.connectors` | Supported, optional | `spec.connectRest.authentication.*.secretRef`, mounted Secret file references in `spec.configs`, KBeacon annotations | Requires Confluent for Kubernetes Connector CRDs to be installed before enabling the watcher. KBeacon models Kubernetes Secret metadata references only and does not call Kafka Connect REST APIs or inspect Secret values. |

## Dependency source types

| Source type | Meaning |
| --- | --- |
| `env.secretKeyRef` | A container environment variable references one key in a Secret. |
| `envFrom.secretRef` | A container imports all keys from a Secret as environment variables. |
| `volumes.secret` | A Pod volume mounts a Secret. |
| `volumes.projected.sources.secret` | A projected volume includes a Secret source. |
| `imagePullSecrets` | A Pod references an image pull Secret directly. |
| `serviceAccount.imagePullSecrets` | A ServiceAccount provides image pull Secret fallback discovery. |
| `ingress.tls` | An Ingress TLS entry references a Secret. |
| `cert-manager.certificate.spec.secretName` | A cert-manager Certificate writes or renews a target Secret. |
| `external-secrets.externalsecret.spec.target.name` | An External Secrets Operator ExternalSecret writes or renews a target Kubernetes Secret. |
| `secrets-store.csi.secretproviderclass.spec.secretObjects.secretName` | A Secrets Store CSI Driver SecretProviderClass syncs or writes a Kubernetes Secret through `spec.secretObjects[*].secretName`. |
| `strimzi.kafkaconnector.spec.config.secrets` | A Strimzi KafkaConnector uses the Strimzi Kubernetes Config Provider Secret syntax in `spec.config` string values. |
| `confluent.connector.spec.connectRest.authentication.secretRef` | A Confluent for Kubernetes Connector references a Kubernetes Secret for Connect REST authentication. |
| `confluent.connector.spec.configs.file.mountedSecret` | A Confluent for Kubernetes Connector references a mounted Kubernetes Secret through `${file:/mnt/secrets/<secret>/...:key}` style config values. |
| `annotation` | A KBeacon explicit dependency annotation declares a Secret dependency. |

## Future or not currently implemented

| Resource | Status | Expected dependency model |
| --- | --- | --- |
| ReplicaSet owner resolution | Planned | Prefer controller workload ownership rather than adding ReplicaSet as a primary output node. |

## Operational notes

Disabling a watcher removes that resource from KBeacon discovery and omits matching chart RBAC when the chart owns RBAC generation.

Optional CRD watchers are disabled by default. Enable them only after the matching CRD is installed in the cluster.

`resourcesToWatch.core.secrets=false` is the low-privilege mode. KBeacon still discovers workload references, but it cannot confirm whether a referenced Secret exists. In that mode, `exists=false` and `resolved=false` mean missing or unobservable.

Secret names, workload names, namespaces, ownership labels, source paths, and dependency relationships can be sensitive operational metadata. Protect Prometheus, Grafana, logs, and the Agent API accordingly.
