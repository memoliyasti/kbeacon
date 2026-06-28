#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${KBEACON_NAMESPACE:-kbeacon-system}"
PORT="${KBEACON_PORT:-18081}"
METRICS_FILE="${KBEACON_METRICS_FILE:-/tmp/kbeacon-demo-metrics.prom}"
SUMMARY_FILE="${KBEACON_METRICS_SUMMARY_FILE:-/tmp/kbeacon-demo-metrics-summary.json}"
PORT_FORWARD_LOG="${KBEACON_PORT_FORWARD_LOG:-/tmp/kbeacon-demo-metrics-port-forward.log}"

echo "Applying blast-radius demo resources..."
./examples/demo-blast-radius/run.sh apply

echo "Waiting for KBeacon rollout..."
kubectl -n "${NAMESPACE}" rollout status deploy/kbeacon

echo "Starting port-forward on 127.0.0.1:${PORT}..."
kubectl -n "${NAMESPACE}" port-forward svc/kbeacon "${PORT}:8080" > "${PORT_FORWARD_LOG}" 2>&1 &
PF_PID="$!"

cleanup() {
  kill "${PF_PID}" >/dev/null 2>&1 || true
  wait "${PF_PID}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

ready="false"
for i in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${PORT}/readyz" >/dev/null 2>&1; then
    ready="true"
    break
  fi
  sleep 1
done

if [ "${ready}" != "true" ]; then
  echo "KBeacon API did not become reachable through port-forward."
  echo "Port-forward log:"
  cat "${PORT_FORWARD_LOG}" || true
  exit 1
fi

sleep 5
curl -fsS "http://127.0.0.1:${PORT}/metrics" > "${METRICS_FILE}"

line_matching() {
  local metric="$1"
  shift
  local lines
  lines="$(grep "^${metric}{" "${METRICS_FILE}" || true)"
  while [ "$#" -gt 0 ]; do
    lines="$(printf "%s\n" "${lines}" | grep "$1" || true)"
    shift
  done
  printf "%s\n" "${lines}" | head -n 1
}

assert_metric() {
  local metric="$1"
  local expected="$2"
  shift 2
  local line
  local actual
  line="$(line_matching "${metric}" "$@")"
  if [ -z "${line}" ]; then
    echo "Missing metric: ${metric} labels=$*"
    exit 1
  fi
  actual="${line##* }"
  if ! awk -v a="${actual}" -v e="${expected}" "BEGIN { exit !((a + 0) == (e + 0)) }"; then
    echo "Metric value mismatch for ${metric}"
    echo "Expected: ${expected}"
    echo "Actual:   ${actual}"
    echo "Line:     ${line}"
    exit 1
  fi
  echo "ok ${metric} expected=${expected} labels=$*"
}

assert_metric kbeacon_secret_affected_workload_count 3 "namespace=\"payments\"" "secret_name=\"payments-db\"" "exists=\"true\""
assert_metric kbeacon_secret_impact_score 46 "namespace=\"payments\"" "secret_name=\"payments-db\"" "exists=\"true\""
assert_metric kbeacon_secret_affected_workload_count 1 "namespace=\"payments\"" "secret_name=\"legacy-payment-token\"" "exists=\"false\""
assert_metric kbeacon_secret_impact_score 38 "namespace=\"payments\"" "secret_name=\"legacy-payment-token\"" "exists=\"false\""
assert_metric kbeacon_unresolved_secret_references 1 "namespace=\"payments\"" "secret_name=\"legacy-payment-token\""
assert_metric kbeacon_workload_dependency_count 3 "namespace=\"payments\"" "workload_name=\"payments-api\""
assert_metric kbeacon_dependency_edges 1 "workload_namespace=\"payments\"" "workload_name=\"payments-api\"" "secret_namespace=\"payments\"" "secret_name=\"legacy-payment-token\"" "resolved=\"false\""

printf "%s\n" "{" > "${SUMMARY_FILE}"
printf "%s\n" "  \"paymentsDbAffectedWorkloads\": 3," >> "${SUMMARY_FILE}"
printf "%s\n" "  \"paymentsDbImpactScore\": 46," >> "${SUMMARY_FILE}"
printf "%s\n" "  \"legacyPaymentTokenAffectedWorkloads\": 1," >> "${SUMMARY_FILE}"
printf "%s\n" "  \"legacyPaymentTokenImpactScore\": 38," >> "${SUMMARY_FILE}"
printf "%s\n" "  \"legacyPaymentTokenUnresolvedReferences\": 1," >> "${SUMMARY_FILE}"
printf "%s\n" "  \"paymentsApiDependencyCount\": 3," >> "${SUMMARY_FILE}"
printf "%s\n" "  \"paymentsApiLegacyTokenEdgeResolved\": false" >> "${SUMMARY_FILE}"
printf "%s\n" "}" >> "${SUMMARY_FILE}"

echo "Demo metrics validation passed."
echo "Metrics file: ${METRICS_FILE}"
echo "Summary file: ${SUMMARY_FILE}"
