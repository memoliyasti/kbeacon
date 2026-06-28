# Alerting

Example Prometheus recording and alert rules are stored at:

    examples/prometheus/rules.yaml

Validate the rules with Prometheus promtool:

    docker run --rm -i \
      --entrypoint=promtool \
      prom/prometheus:v3.1.0 \
      check rules /dev/stdin < examples/prometheus/rules.yaml

KBeacon intentionally does not send notifications directly. Use Prometheus Alertmanager or Grafana Alerting.
