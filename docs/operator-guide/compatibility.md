# Compatibility matrix

KBeacon is tested as a lightweight Agent and Helm chart. Optional integrations depend on third-party CRDs and their Kubernetes version support.

## Kubernetes versions

| Kubernetes | Status | Notes |
| --- | --- | --- |
| 1.26 | Supported for core KBeacon resources | Use compatible versions of optional CRD providers. Newer upstream charts can require 1.30+. |
| 1.28 | Supported for core KBeacon resources | Recommended baseline for broader optional CRD testing. |
| 1.30+ | Supported target for latest optional CRD provider charts | Best target for current cert-manager, External Secrets, Secrets Store CSI Driver, and monitoring chart combinations. |

## Optional resource integrations

| Integration | KBeacon behavior | Requirement | Failure mode if absent |
| --- | --- | --- | --- |
| Prometheus Operator ServiceMonitor | Renders ServiceMonitor when enabled | `servicemonitors.monitoring.coreos.com` CRD | Leave `serviceMonitor.enabled=false` or use scrape annotations. |
| PrometheusRule examples | Rules can be installed by platform teams | `prometheusrules.monitoring.coreos.com` CRD | Use static Prometheus rule files instead. |
| cert-manager Certificate | Discovers `spec.secretName` when enabled | `certificates.cert-manager.io` CRD | Optional cache reports disabled or CRD unavailable. |
| External Secrets Operator ExternalSecret | Discovers target Secret when enabled | `externalsecrets.external-secrets.io` CRD | Optional cache reports disabled or CRD unavailable. |
| Secrets Store CSI Driver SecretProviderClass | Discovers `secretObjects[*].secretName` when enabled | `secretproviderclasses.secrets-store.csi.x-k8s.io` CRD | Optional cache reports disabled or CRD unavailable. |
| Strimzi KafkaConnector | Discovers Kubernetes config provider Secret refs when enabled | `kafkaconnectors.kafka.strimzi.io` CRD | Optional cache reports disabled or CRD unavailable. |
| Confluent Connector | Discovers supported connector Secret refs when enabled | `connectors.platform.confluent.io` CRD | Optional cache reports disabled or CRD unavailable. |

## Local Minikube note

Older Minikube profiles can run Kubernetes 1.26. Some current third-party Helm charts require newer Kubernetes APIs. When optional CRD chart installation fails in local testing, first check the cluster version and install a provider chart version compatible with that Kubernetes release.

```bash
kubectl version
kubectl get crd | grep -E "cert-manager.io|external-secrets.io|secrets-store.csi.x-k8s.io|kafka.strimzi.io|platform.confluent.io|monitoring.coreos.com"
```
