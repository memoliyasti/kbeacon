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
