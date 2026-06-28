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
