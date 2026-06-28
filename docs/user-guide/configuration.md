
# Configuration

KBeacon is configured through Helm values and the generated Agent config.

## Important values

- `cluster.name`: logical cluster identity.
- `discovery.defaultMode`: `infer`, `explicit`, `hybrid`, or `disabled`.
- `discovery.namespaces.include`: namespace allow-list.
- `discovery.namespaces.exclude`: namespace deny-list.
- `resourcesToWatch`: implemented informer enablement.
- `metrics.edge.enabled`: detailed edge metric cardinality guard.
- `metrics.runtime.enabled`: runtime metric collection.

## Namespace filtering

    discovery:
      namespaces:
        include:
          - payments
        exclude:
          - kube-system
          - kube-public
          - kube-node-lease

`include: []` means all namespaces are eligible unless excluded.

## Low-privilege mode

    resourcesToWatch:
      core:
        secrets: false

KBeacon still discovers workload-to-Secret references from workload specs and annotations. Because Secret objects are not observed, referenced Secrets are reported as `exists=false`.

## Metrics cardinality

Detailed edge metrics include workload and Secret names as labels.

For large clusters or shared Prometheus environments, disable `kbeacon_dependency_edges`:

    metrics:
      edge:
        enabled: false
      runtime:
        enabled: true

Aggregate impact metrics and the Agent API remain available.

## Implemented resource watchers

    resourcesToWatch:
      core:
        secrets: true
        pods: true
      apps:
        deployments: true
        statefulSets: true
        daemonSets: true
      batch:
        jobs: true
        cronJobs: true
