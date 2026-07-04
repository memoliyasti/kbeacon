# Alerting

Example Prometheus recording and alert rules are stored at:

    examples/prometheus/rules.yaml

Validate the rules with Prometheus promtool:

    docker run --rm -i \
      --entrypoint=promtool \
      prom/prometheus:v3.1.0 \
      check rules /dev/stdin < examples/prometheus/rules.yaml

KBeacon intentionally does not send notifications directly. Use Prometheus Alertmanager or Grafana Alerting.

## Alert runbooks

Each alert rule includes a `runbook_url` annotation that points to the operator runbook page. See [Alert runbooks](../operator-guide/runbooks.md).

When using Alertmanager, include the `runbook_url` annotation in notification templates so responders can jump directly from the alert to the triage steps.
