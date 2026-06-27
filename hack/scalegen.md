# Scale Generator Design

A future `hack/scalegen` tool should generate synthetic namespaces, Secrets, and workloads to benchmark KBeacon extraction and metric rendering.

Example target interface:

```bash
go run ./hack/scalegen \
  --namespaces=100 \
  --workloads=10000 \
  --secrets=5000 \
  --edges=50000 \
  --output=/tmp/kbeacon-scale
```

The generated fixtures should support:

- random but deterministic dependency edges;
- controllable fan-out distribution;
- namespace and team distribution;
- optional unresolved references;
- Kubernetes YAML and direct JSON graph fixture output.
