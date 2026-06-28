# Configuration

KBeacon is configured through the Helm chart values file and the generated Agent configuration.

Important values:

- `cluster.name`: logical cluster identity.
- `discovery.defaultMode`: `infer`, `explicit`, `hybrid`, or `disabled`.
- `discovery.namespaces.include`: namespace allow-list.
- `discovery.namespaces.exclude`: namespace deny-list.
- `resourcesToWatch`: resource informer enablement.
- `metrics.runtime.enabled`: runtime metric collection.

Example namespace filtering:

    discovery:
      namespaces:
        include:
          - payments
        exclude:
          - kube-system
          - kube-public
          - kube-node-lease

## Metrics cardinality

Detailed edge metrics are useful, but they include workload and Secret names as labels.

For large clusters or shared Prometheus environments, disable `kbeacon_dependency_edges`:

    metrics:
      edge:
        enabled: false
      runtime:
        enabled: true

Aggregate impact metrics and the Agent API remain available.
