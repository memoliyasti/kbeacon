# Alerting

Prometheus alerting and recording rules are available in:

    examples/prometheus/rules.yaml

Validate them with:

    docker run --rm -i \
      --entrypoint=promtool \
      prom/prometheus:v3.1.0 \
      check rules /dev/stdin < examples/prometheus/rules.yaml

The example rules cover:

- Agent down
- cache not synced
- graph rebuild latency
- Secret changed with workload impact
- high impact Secrets
- large Secret fan-out
- unresolved Secret references
- no dependencies discovered
