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

Use a classic GitHub PAT with read:packages only for Kubernetes image pulls from private GHCR packages. Do not reuse maintainer admin tokens for clusters. Rotate any token that has been pasted into a terminal transcript, issue, pull request, chat, or documentation.

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
