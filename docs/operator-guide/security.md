# Security operations

KBeacon uses read-only Kubernetes access and does not export Secret values.

Operational recommendations:

- Keep the Agent API internal.
- Treat Secret names as sensitive metadata.
- Restrict access to Prometheus, Mimir, Grafana, and Agent logs.
- Use private registry pull secrets when the GHCR package is private.
- Rotate registry tokens used for image pulls.
- Prefer digest-pinned images in production.
