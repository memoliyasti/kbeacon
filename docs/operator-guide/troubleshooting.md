# Troubleshooting

This guide helps operators diagnose common KBeacon deployment, discovery, metrics, dashboard, and release verification issues.

KBeacon is intentionally read-only. Most failures fall into one of five areas:

- Helm values or rendered manifests;
- Kubernetes RBAC and informer access;
- workload discovery configuration;
- Prometheus scraping and label handling;
- Grafana dashboard data source or query configuration.

## Quick triage

Start with the deployment, logs, readiness endpoint, and graph summary.

```bash
kubectl -n kbeacon-system get deploy,pod,svc,serviceaccount
kubectl -n kbeacon-system rollout status deploy/kbeacon
kubectl -n kbeacon-system logs deploy/kbeacon --tail=100
kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
curl -sS http://127.0.0.1:8081/readyz | jq
curl -sS http://127.0.0.1:8081/api/v1/config | jq
```

A healthy Agent reports `status` as `ready` and shows all enabled informer caches as synced.

## Helm install or upgrade fails

Validate values before applying them to a cluster.

```bash
helm lint ./charts/kbeacon --set cluster.name=ci
helm template kbeacon ./charts/kbeacon \
  --namespace kbeacon-system \
  --set cluster.name=ci \
  --set dashboards.enabled=true \
  > /tmp/kbeacon-rendered.yaml
```

Common causes:

- `cluster.name` is empty;
- unsupported values are rejected by `values.schema.json`;
- Prometheus Operator CRDs are missing while `serviceMonitor.enabled=true`;
- namespace-scoped RBAC is used without matching namespace include values;
- image repository, tag, digest, or pull policy does not match the deployment environment.

## Pod is not starting

Check Pod events and the effective image.

```bash
kubectl -n kbeacon-system describe pod -l app.kubernetes.io/name=kbeacon
kubectl -n kbeacon-system get pod -l app.kubernetes.io/name=kbeacon \
  -o jsonpath={range .items[*]}{.metadata.name}{"  "}{.spec.containers[0].image}{"  "}{.status.phase}{"\n"}{end}
```

Typical causes:

| Symptom | Likely cause | Action |
| --- | --- | --- |
| `ImagePullBackOff` | private registry, wrong tag, or missing image pull Secret | verify `image.repository`, `image.tag`, `image.digest`, and `imagePullSecrets` |
| container exits immediately | invalid config or invalid duration value | inspect Agent logs and rendered ConfigMap |
| readiness never becomes ready | informer cache cannot sync | inspect RBAC and API server access |
| rollout uses an old image | old ReplicaSet still terminating or image pull policy | check Pods by image and wait for old Pods to exit |

## Agent logs show Kubernetes watch or list errors

KBeacon uses Kubernetes informers. If list or watch calls fail, check RBAC and namespace scope.

```bash
kubectl auth can-i list pods --as system:serviceaccount:kbeacon-system:kbeacon -A
kubectl auth can-i list secrets --as system:serviceaccount:kbeacon-system:kbeacon -A
kubectl auth can-i list deployments.apps --as system:serviceaccount:kbeacon-system:kbeacon -A
kubectl auth can-i list jobs.batch --as system:serviceaccount:kbeacon-system:kbeacon -A
```

For namespace-scoped installs, run the same checks with `-n <namespace>` instead of `-A`.

Low-privilege mode intentionally removes Secret RBAC:

```yaml
resourcesToWatch:
  core:
    secrets: false
```

In that profile, Secret objects are unobservable. Referenced Secrets are represented with `exists=false`, and dependency edges are marked `resolved=false`.

## Readiness reports unsynced caches

Inspect `/readyz`.

```bash
curl -sS http://127.0.0.1:8081/readyz | jq
```

Use the resource name in the readiness response to decide the next check.

| Resource | Check |
| --- | --- |
| `Secret` | Secret RBAC, or confirm low-privilege mode is intentional |
| `Pod` | core Pod list and watch permissions |
| `Deployment`, `StatefulSet`, `DaemonSet` | apps API list and watch permissions |
| `Job`, `CronJob` | batch API list and watch permissions |

Disabled resources are not started as informers and should not block readiness.

## No workloads or Secrets appear in the API

Check the graph summary and filters.

```bash
curl -sS http://127.0.0.1:8081/api/v1/config | jq
curl -sS http://127.0.0.1:8081/api/v1/workloads | jq
curl -sS http://127.0.0.1:8081/api/v1/secrets | jq
```

Likely causes:

- `discovery.namespaces.include` does not include the workload namespace;
- `discovery.namespaces.exclude` removes the namespace;
- relevant resource watchers are disabled;
- workloads are marked with `kbeacon.io/enabled: "false"`;
- workloads use `kbeacon.io/discovery-mode: disabled`;
- workloads do not reference Secrets in supported fields and have no explicit KBeacon annotations.

## Expected Secret dependency is missing

KBeacon currently discovers Secret references from these sources:

- `env.valueFrom.secretKeyRef`;
- `envFrom.secretRef`;
- `volumes.secret`;
- `imagePullSecrets`;
- `kbeacon.io/watch-secrets`;
- `kbeacon.io/watch-secrets-json`.

Check the workload spec and annotations.

```bash
kubectl -n <namespace> get deploy <name> -o yaml
kubectl -n <namespace> get pod <name> -o yaml
```

For dependencies that are not visible in Pod specs, use explicit annotations.

```yaml
metadata:
  annotations:
    kbeacon.io/discovery-mode: hybrid
    kbeacon.io/watch-secrets: payments-db,shared/platform-ca
```

Use `kbeacon.io/ignore-secrets` carefully. Ignored references are removed from the graph.

## All dependencies are unresolved

This usually means one of these conditions is true:

- low-privilege mode is enabled and Secret objects are not watched;
- Secret RBAC is missing;
- referenced Secrets do not exist;
- Secret namespace filters exclude the namespace;
- the Agent has not synced the Secret cache yet.

Check whether Secret watching is enabled.

```bash
helm get values kbeacon -n kbeacon-system -a | grep -A5 resourcesToWatch
kubectl -n kbeacon-system logs deploy/kbeacon --tail=100
curl -sS http://127.0.0.1:8081/readyz | jq
```

## Metrics are not scraped

Check the Service and scrape integration.

```bash
kubectl -n kbeacon-system get svc kbeacon -o yaml
kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
curl -sS http://127.0.0.1:8081/metrics | head
```

For Prometheus Operator, verify the ServiceMonitor.

```bash
kubectl -n kbeacon-system get servicemonitor kbeacon -o yaml
```

For annotation-based scraping, confirm Service annotations.

```yaml
prometheus:
  scrapeAnnotations:
    enabled: true
    target: service
```

If using static scrape configuration, verify the target DNS name and port.

```yaml
scrape_configs:
  - job_name: kbeacon-agent
    honor_labels: true
    metrics_path: /metrics
    static_configs:
      - targets:
          - kbeacon.kbeacon-system.svc.cluster.local:8080
```

## Prometheus labels look wrong

KBeacon metrics include domain labels such as `cluster`, `namespace`, `secret_name`, and `workload_name`.

Prometheus scrape labels can conflict with exported labels. Keep `honor_labels=true` for static scrape configs or `serviceMonitor.honorLabels=true` for Prometheus Operator.

```yaml
serviceMonitor:
  honorLabels: true
```

If honor labels are disabled, review dashboard queries because Prometheus may rename conflicting labels to exported labels.

## Grafana dashboards are empty

Check the data path in this order:

1. Agent `/metrics` returns KBeacon metrics.
2. Prometheus scrapes the Agent target.
3. Grafana data source can query Prometheus or Mimir.
4. Dashboard variables select the correct `job` and `cluster`.
5. Edge-level panels require `metrics.edge.enabled=true`.

Useful PromQL checks:

```promql
up{job=~".*kbeacon.*"}
kbeacon_cluster_dependency_count
kbeacon_cluster_secret_count
kbeacon_cluster_workload_count
kbeacon_dependency_edges
```

If `kbeacon_dependency_edges` is absent and edge metrics are disabled, Node Graph panels and edge detail tables will be empty by design.

## High cardinality concerns

`kbeacon_dependency_edges` includes workload and Secret names as labels.

Disable detailed edge metrics when Prometheus cardinality budget is more important than edge-level graph exploration.

```yaml
metrics:
  edge:
    enabled: false
```

Aggregate metrics and the read-only Agent API remain available.

## API access errors

The Agent API is intended for internal or controlled platform access.

Check the Service, Pod, and HTTP listener.

```bash
kubectl -n kbeacon-system get svc,pod -l app.kubernetes.io/name=kbeacon
kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
curl -sS http://127.0.0.1:8081/api/v1 | jq
```

Compatibility aliases under `/api/...` exist, but `/api/v1/...` is the preferred API path.

## Release image verification

Public release images should pull without GHCR authentication.

```bash
docker logout ghcr.io || true
docker pull ghcr.io/memoliyasti/kbeacon:0.2.4
docker pull ghcr.io/memoliyasti/kbeacon:v0.2.4
```

Images published after Cosign signing was enabled can be verified with the GitHub Actions OIDC identity used by the publishing workflow.

```bash
cosign verify \
  --certificate-identity https://github.com/memoliyasti/kbeacon/.github/workflows/ci.yaml@refs/heads/main \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/memoliyasti/kbeacon@sha256:<digest>
```

## Useful validation commands

Run the same local validation targets used by CI.

```bash
make validate-ci
make helm-schema-lint
./hack/validate-dashboards.sh
mkdocs build --strict
```

## Related documentation

- Installation: `docs/user-guide/installation.md`
- Configuration: `docs/user-guide/configuration.md`
- Discovery modes: `docs/user-guide/discovery-modes.md`
- Helm reference: `docs/reference/helm.md`
- Metrics reference: `docs/reference/metrics.md`
- Prometheus operations: `docs/operations/prometheus.md`
- Dashboard guide: `docs/user-guide/dashboards.md`
