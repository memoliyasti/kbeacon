## Summary

<!-- What changed and why? -->

## Type of change

- [ ] Bug fix
- [ ] Feature
- [ ] Documentation
- [ ] Helm chart
- [ ] Metrics/API contract
- [ ] CI/release
- [ ] Refactor

## Validation

- [ ] `go fmt ./...`
- [ ] `go test ./...`
- [ ] `go build -o ./bin/kbeacon-agent ./cmd/kbeacon-agent`
- [ ] `helm lint ./charts/kbeacon --set cluster.name=ci`
- [ ] `helm template kbeacon ./charts/kbeacon --namespace kbeacon-system --set cluster.name=ci --set dashboards.enabled=true`
- [ ] `docker run --rm -i --entrypoint=promtool prom/prometheus:v3.1.0 check rules /dev/stdin < examples/prometheus/rules.yaml`
- [ ] `mkdocs build --strict`

## Safety checklist

- [ ] This change does not expose Kubernetes Secret values.
- [ ] This change keeps Kubernetes RBAC read-only.
- [ ] New Prometheus labels are bounded-cardinality and documented.
- [ ] API or metric contract changes are documented.
- [ ] Helm defaults remain safe for production.
- [ ] Security-sensitive examples use placeholders only.

## Notes for reviewers

<!-- Anything reviewers should focus on? -->
