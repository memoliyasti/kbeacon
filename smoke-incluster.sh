#!/usr/bin/env bash
set -euo pipefail

PROM_URL="${PROM_URL:-http://localhost:9090}"

echo "Checking Prometheus ready endpoint"
curl -sS "${PROM_URL}/-/ready"
echo

echo "Checking KBeacon scrape target"
curl -sS -G "${PROM_URL}/api/v1/query" \
  --data-urlencode 'query=up{job="kbeacon-agent"}' | jq

echo "Checking dependency count"
curl -sS -G "${PROM_URL}/api/v1/query" \
  --data-urlencode 'query=kbeacon_cluster_dependency_count{job="kbeacon-agent"}' | jq

echo "Checking app-db-secret impact"
curl -sS -G "${PROM_URL}/api/v1/query" \
  --data-urlencode 'query=kbeacon_secret_impact_score{job="kbeacon-agent",namespace="kbeacon-demo",secret_name="app-db-secret"}' | jq

echo "Checking graph update metrics"
curl -sS -G "${PROM_URL}/api/v1/query" \
  --data-urlencode 'query=kbeacon_graph_update_duration_seconds_count{job="kbeacon-agent"}' | jq

echo "Checking watch event metrics"
curl -sS -G "${PROM_URL}/api/v1/query" \
  --data-urlencode 'query=kbeacon_kubernetes_watch_events_total{job="kbeacon-agent",resource="Deployment"}' | jq
