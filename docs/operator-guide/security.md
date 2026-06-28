# Security model

KBeacon is read-only by design.

## Secret data

KBeacon must not expose Secret values.

It may expose:

- Secret names;
- Secret namespaces;
- Secret types;
- dependency relationships;
- owner team metadata;
- criticality metadata.

Treat this metadata as sensitive.

## Kubernetes permissions

The Helm chart grants read-only `get`, `list`, and `watch` permissions for enabled resources.

## Agent API

The Agent API is intended for in-cluster or trusted internal access. Do not expose it publicly.

## Registry authentication

If GHCR packages are private, use a Kubernetes image pull secret with `read:packages` permission.
