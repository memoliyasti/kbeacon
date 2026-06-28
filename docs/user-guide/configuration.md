# Configuration

KBeacon is configured through Helm values rendered into an Agent config file.

## Cluster identity

    cluster:
      name: prod-eu-1
      environment: prod
      region: eu

## Namespace filters

    discovery:
      namespaces:
        include: []
        exclude:
          - kube-system
          - kube-public
          - kube-node-lease

An empty include list means all namespaces are eligible unless excluded.

## Discovery mode

    discovery:
      defaultMode: hybrid

Supported values:

- infer
- explicit
- hybrid
- disabled

## Resource watchers

    resourcesToWatch:
      core:
        secrets: true
        pods: true
      apps:
        deployments: true
        statefulSets: true
        daemonSets: true
        replicaSets: false
      batch:
        jobs: true
        cronJobs: true

Disabled resources appear as optional in readiness responses.
