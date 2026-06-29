
# Configuration

KBeacon is configured through Helm values and the generated Agent config.

## Important values

- `cluster.name`: logical cluster identity.
- `discovery.defaultMode`: `infer`, `explicit`, `hybrid`, or `disabled`.
- `discovery.namespaces.include`: namespace allow-list.
- `discovery.namespaces.exclude`: namespace deny-list.
- `discovery.metadataLabels`: fallback label keys for owner, service, environment, and criticality metadata.
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


## Existing workload label fallback

KBeacon annotations are optional for workload ownership and classification metadata.

When a workload does not define `kbeacon.io/owner-team`, `kbeacon.io/service`, `kbeacon.io/environment`, or `kbeacon.io/criticality`, KBeacon can read existing Kubernetes labels instead.

Default label keys:

    discovery:
      metadataLabels:
        enabled: true
        ownerTeam:
          - app.kubernetes.io/team
          - team
          - owner-team
          - ownerTeam
          - technical-owner
          - technicalOwner
          - business-owner
          - businessOwner
        service:
          - app.kubernetes.io/name
          - app
          - service
          - service-name
          - serviceName
        environment:
          - app.kubernetes.io/environment
          - environment
          - env
          - stage
        criticality:
          - app.kubernetes.io/criticality
          - criticality
          - priority
          - tier
          - slo-tier

Precedence:

1. KBeacon annotations.
2. Workload object labels.
3. Pod template labels, when `discovery.readPodTemplateAnnotations=true`.

Changing workload object labels or annotations does not change the Pod template and therefore does not trigger a Deployment rollout. KBeacon annotations are still useful when teams need an explicit override or a dependency that cannot be inferred from the Pod spec.
