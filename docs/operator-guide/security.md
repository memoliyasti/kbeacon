# Security operations

KBeacon uses read-only Kubernetes access and does not export Secret values. It still exposes Secret dependency metadata, which should be treated as sensitive operational data.

## Recommended deployment controls

- Keep the KBeacon Service internal.
- Restrict access to the Agent API.
- Restrict access to Prometheus, Mimir, and Grafana.
- Use NetworkPolicy where possible.
- Use namespace filters when full-cluster visibility is not required.
- Keep image pull credentials scoped and rotated.
- Prefer public GHCR packages when the project is intentionally public.
- Prefer digest pinning for controlled production rollouts.

## Private registry tokens

The official KBeacon GHCR image is public and does not require a Kubernetes image pull Secret. For private forks or private registries, use a narrowly scoped pull credential, do not reuse maintainer admin tokens for clusters, and rotate any token that has been pasted into a terminal transcript, issue, pull request, chat, or documentation.

## Metadata sensitivity

Even without Secret values, the following can be sensitive:

- Secret names;
- namespaces;
- workload names;
- team ownership labels;
- dependency edges;
- impact scores.

Protect dashboards and metrics accordingly.

## Low-privilege mode

By default, KBeacon can watch Secret metadata so it can resolve whether a referenced Secret exists and read safe metadata such as type and annotations. Kubernetes RBAC does not separate Secret metadata from Secret data, so this permission is sensitive even though KBeacon does not export Secret values.

Use low-privilege mode when Secret read access is not acceptable:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --create-namespace \
      --set cluster.name=prod-eu-1 \
      --set resourcesToWatch.core.secrets=false

In low-privilege mode, KBeacon discovers workload references but cannot confirm Secret existence. Treat `exists=false` and `resolved=false` as "missing or unobservable".

## Metric label sensitivity

KBeacon does not export Secret values, but Secret names, workload names, namespaces, owner teams, and criticality labels can still be sensitive.

For stricter environments:

- restrict access to Prometheus, Mimir, Grafana, and the Agent API;
- keep the Agent Service internal;
- disable detailed edge metrics with `metrics.edge.enabled=false`;
- use low-privilege mode with `resourcesToWatch.core.secrets=false` when Secret object reads are not allowed.
