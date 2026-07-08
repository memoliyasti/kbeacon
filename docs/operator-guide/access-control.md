# Access control

KBeacon exposes a read-only Agent API. The API does not return Kubernetes Secret values, but Secret names and dependency metadata can still be sensitive.

## Recommended access model

- Keep the KBeacon Service as `ClusterIP`.
- Do not expose the Agent API directly to the public internet.
- Prefer kube-native CLI access through the Kubernetes API server Service proxy.
- Grant `services/proxy` only to platform users or automation that should read KBeacon dependency metadata.
- Protect Prometheus, Grafana, Mimir, and logs with the same sensitivity as dependency metadata.

## Minimal RBAC for kube-native CLI users

The `kbeacon` CLI talks to the in-cluster Agent through the Kubernetes Service proxy. A restricted user needs permission to read the proxied Service endpoint.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kbeacon-cli-reader
  namespace: kbeacon-system
rules:
  - apiGroups: [""]
    resources: ["services/proxy"]
    resourceNames: ["kbeacon", "kbeacon:http"]
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kbeacon-cli-reader-platform
  namespace: kbeacon-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kbeacon-cli-reader
subjects:
  - kind: Group
    name: platform-engineers
    apiGroup: rbac.authorization.k8s.io
```

Validate access with:

```bash
kubectl auth can-i get services/proxy -n kbeacon-system --resource-name kbeacon
kbeacon --namespace kbeacon-system ready
kbeacon --namespace kbeacon-system get config
```

## Grafana data links

Grafana dashboard data links should point to an internal, authenticated Agent API endpoint or to a controlled gateway. Do not embed unauthenticated public Agent API URLs in shared dashboards.
