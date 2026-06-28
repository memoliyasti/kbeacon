## Summary

Describe the change and why it is needed.

## Type of change

- [ ] Bug fix
- [ ] Feature
- [ ] Documentation
- [ ] Helm/chart change
- [ ] Metrics/API change
- [ ] CI/release change

## Validation

- [ ] `make validate-ci`
- [ ] `make demo-dry-run`

- [ ] `go fmt ./...`
- [ ] `go test ./...`
- [ ] `go build -o ./bin/kbeacon-agent ./cmd/kbeacon-agent`
- [ ] `helm lint ./charts/kbeacon --set cluster.name=ci`
- [ ] `helm template kbeacon ./charts/kbeacon --namespace kbeacon-system --set cluster.name=ci --set dashboards.enabled=true`
- [ ] `helm template kbeacon ./charts/kbeacon --namespace payments --set cluster.name=ci --set rbac.scope=namespace --set discovery.namespaces.include={payments}`
- [ ] `helm template kbeacon ./charts/kbeacon --namespace kbeacon-system --set cluster.name=ci --set metrics.edge.enabled=false`
- [ ] `helm template kbeacon ./charts/kbeacon --namespace kbeacon-system --set cluster.name=ci --set resourcesToWatch.core.secrets=false`
- [ ] `promtool check rules examples/prometheus/rules.yaml`

## Security checklist

- [ ] This change does not expose Secret values.
- [ ] This change keeps Kubernetes permissions read-only.
- [ ] This change does not add unbounded metric labels.
- [ ] `make stale-check` passes.
- [ ] Documentation has been updated if behavior changes.

## Notes

Add any rollout, compatibility, or follow-up notes.
