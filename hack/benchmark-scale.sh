#!/usr/bin/env bash
set -euo pipefail

# Live scale benchmark harness for KBeacon.
# It generates deterministic fixture namespaces, applies them to the current
# Kubernetes context, waits until the Agent API sees them, then records API,
# metric, and best-effort Kubernetes resource observations.
#
# Required commands: kubectl, curl, jq, python3
# Optional command: helm, if your local workflow uses Helm before running this.

OUT_ROOT="${KBEACON_SCALE_BENCHMARK_OUT:-/tmp/kbeacon-scale-benchmark}"
LEVELS="${KBEACON_SCALE_LEVELS:-100 1000}"
SECRET_RATIO="${KBEACON_SCALE_SECRET_RATIO:-4}"
NAMESPACE_PREFIX="${KBEACON_SCALE_NAMESPACE_PREFIX:-kbeacon-scale}"
KBEACON_URL="${KBEACON_URL:-http://127.0.0.1:8081}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://127.0.0.1:9090}"
KBEACON_NAMESPACE="${KBEACON_NAMESPACE:-kbeacon-system}"
KBEACON_DEPLOYMENT="${KBEACON_DEPLOYMENT:-kbeacon}"
WAIT_TRIES="${KBEACON_SCALE_WAIT_TRIES:-60}"
WAIT_SLEEP="${KBEACON_SCALE_WAIT_SLEEP:-5}"
RETAIN="${KBEACON_SCALE_RETAIN:-false}"
APPLY="${KBEACON_SCALE_APPLY:-true}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

require_cmd kubectl
require_cmd curl
require_cmd jq
require_cmd python3

now_iso() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

now_ns() {
  python3 - <<'PYTIME'
import time
print(time.time_ns())
PYTIME
}

curl_json() {
  local url="$1"
  curl -fsS --max-time 20 "$url"
}

curl_time_seconds() {
  local url="$1"
  curl -fsS -o /dev/null -w '%{time_total}' --max-time 60 "$url"
}

json_number_or_null() {
  local value="$1"
  if [[ -z "${value}" ]]; then
    printf 'null'
  else
    jq -n --argjson v "${value}" '$v'
  fi
}

json_string_or_null() {
  local value="$1"
  if [[ -z "${value}" ]]; then
    printf 'null'
  else
    jq -n --arg v "${value}" '$v'
  fi
}

prom_query_value() {
  local query="$1"
  curl -fsSG --max-time 20 "${PROMETHEUS_URL}/api/v1/query" \
    --data-urlencode "query=${query}" 2>/dev/null \
    | jq -r '.data.result[0].value[1] // empty' 2>/dev/null || true
}

metrics_summary_json() {
  local metrics_url="${KBEACON_URL}/metrics"
  local metrics_file="$1"

  if ! curl -fsS --max-time 30 "${metrics_url}" > "${metrics_file}"; then
    jq -n '{sampleCount:null, graphUpdateCount:null, graphUpdateAvgSeconds:null}'
    return
  fi

  python3 - "${metrics_file}" <<'PYMETRICS'
import json
import sys
from pathlib import Path

path = Path(sys.argv[1])
sample_count = 0
sum_value = None
count_value = None

for raw in path.read_text(encoding='utf-8').splitlines():
    line = raw.strip()
    if not line or line.startswith('#'):
        continue
    sample_count += 1
    if line.startswith('kbeacon_graph_update_duration_seconds_sum'):
        try:
            sum_value = float(line.split()[-1])
        except Exception:
            pass
    elif line.startswith('kbeacon_graph_update_duration_seconds_count'):
        try:
            count_value = float(line.split()[-1])
        except Exception:
            pass

avg = None
if sum_value is not None and count_value not in (None, 0):
    avg = sum_value / count_value

print(json.dumps({
    'sampleCount': sample_count,
    'graphUpdateCount': count_value,
    'graphUpdateAvgSeconds': avg,
}, sort_keys=True))
PYMETRICS
}

kubectl_top_json() {
  local output
  output="$(kubectl -n "${KBEACON_NAMESPACE}" top pod -l app.kubernetes.io/name=kbeacon --no-headers 2>/dev/null || true)"
  if [[ -z "${output}" ]]; then
    jq -n '{available:false, cpu:null, memory:null}'
    return
  fi

  python3 - <<'PYTOP' "${output}"
import json
import sys
raw = sys.argv[1].splitlines()[0].split()
print(json.dumps({
    'available': True,
    'pod': raw[0] if len(raw) > 0 else None,
    'cpu': raw[1] if len(raw) > 1 else None,
    'memory': raw[2] if len(raw) > 2 else None,
}, sort_keys=True))
PYTOP
}

wait_for_api() {
  for i in $(seq 1 "${WAIT_TRIES}"); do
    if curl_json "${KBEACON_URL}/readyz" | jq -e '.status == "ready"' >/dev/null 2>&1; then
      return 0
    fi
    echo "waiting for KBeacon API readiness ${i}/${WAIT_TRIES}" >&2
    sleep "${WAIT_SLEEP}"
  done
  echo "KBeacon API did not become ready at ${KBEACON_URL}" >&2
  exit 1
}

wait_for_workload_total() {
  local ns="$1"
  local expected="$2"
  for i in $(seq 1 "${WAIT_TRIES}"); do
    local total
    total="$(curl_json "${KBEACON_URL}/api/v1/workloads?namespace=${ns}&limit=1" | jq -r '.pagination.total // 0' 2>/dev/null || echo 0)"
    if [[ "${total}" -ge "${expected}" ]]; then
      echo "KBeacon graph observed namespace=${ns} workloads=${total} try=${i}" >&2
      return 0
    fi
    echo "waiting for graph namespace=${ns} workloads=${total}/${expected} try=${i}/${WAIT_TRIES}" >&2
    sleep "${WAIT_SLEEP}"
  done
  echo "KBeacon graph did not observe expected workload total for namespace=${ns}" >&2
  exit 1
}

make_report_markdown() {
  local summary_json="$1"
  local summary_md="$2"
  python3 - "${summary_json}" "${summary_md}" <<'PYMD'
import json
import sys
from pathlib import Path

summary = json.loads(Path(sys.argv[1]).read_text(encoding='utf-8'))
rows = []
for item in summary['levels']:
    rows.append([
        str(item['workloads']),
        str(item['secrets']),
        str(item['expectedEdges']),
        str(item['observed']['workloads']),
        str(item['observed']['dependencyMapEdges']),
        f"{item['apiSeconds']['workloadsList']:.4f}",
        f"{item['apiSeconds']['dependencyMap']:.4f}",
        str(item['metrics']['sampleCount']),
        '' if item['metrics']['graphUpdateAvgSeconds'] is None else f"{item['metrics']['graphUpdateAvgSeconds']:.6f}",
        '' if item.get('prometheusGraphRebuildP95Seconds') in (None, '') else str(item['prometheusGraphRebuildP95Seconds']),
        '' if not item['kubectlTop'].get('available') else item['kubectlTop'].get('memory', ''),
    ])

out = []
out.append('# KBeacon scale benchmark report')
out.append('')
out.append(f"Generated at: `{summary['generatedAt']}`")
out.append(f"KBeacon URL: `{summary['kbeaconUrl']}`")
out.append(f"Prometheus URL: `{summary['prometheusUrl']}`")
out.append('')
out.append('| Workloads | Secrets | Expected edges | Observed workloads | Observed map edges | /workloads seconds | /dependency-map seconds | Metric samples | Graph avg seconds | Prometheus p95 seconds | Pod memory |')
out.append('| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |')
for row in rows:
    out.append('| ' + ' | '.join(row) + ' |')
out.append('')
out.append('Notes: pod memory requires metrics-server. Prometheus p95 requires a reachable Prometheus API and recent scrape samples.')
Path(sys.argv[2]).write_text('\n'.join(out) + '\n', encoding='utf-8')
PYMD
}

wait_for_api

RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${OUT_ROOT}/reports/${RUN_ID}"
FIXTURE_ROOT="${OUT_ROOT}/fixtures/${RUN_ID}"
mkdir -p "${RUN_DIR}" "${FIXTURE_ROOT}"

summary_levels_file="${RUN_DIR}/levels.jsonl"
: > "${summary_levels_file}"

for workloads in ${LEVELS}; do
  if ! [[ "${workloads}" =~ ^[0-9]+$ ]]; then
    echo "invalid workload level: ${workloads}" >&2
    exit 1
  fi

  secrets=$(( (workloads + SECRET_RATIO - 1) / SECRET_RATIO ))
  if [[ "${secrets}" -lt 1 ]]; then
    secrets=1
  fi

  namespace="${NAMESPACE_PREFIX}-${workloads}"
  fixture_dir="${FIXTURE_ROOT}/${workloads}"
  level_dir="${RUN_DIR}/${workloads}"
  mkdir -p "${level_dir}"

  echo "===== scale level workloads=${workloads} secrets=${secrets} namespace=${namespace} ====="
  ./hack/generate-scale-fixture.sh "${fixture_dir}" "${namespace}" "${secrets}" "${workloads}" | tee "${level_dir}/fixture.log"

  expected_edges="$(jq -r '.expectedEdges // .expectedDependencyEdges // .expected_edges // .expectedEdges // .expectedDependencyEdges // .expected_edges // .expectedEdges // .expected_dependency_edges // 0' "${fixture_dir}/expected-summary.json" 2>/dev/null || echo 0)"

  if [[ "${APPLY}" == "true" ]]; then
    kubectl apply -f "${fixture_dir}/namespace.yaml" | tee "${level_dir}/apply.log"
    kubectl apply -f "${fixture_dir}/secrets.yaml" | tee -a "${level_dir}/apply.log"
    kubectl apply -f "${fixture_dir}/workloads.yaml" | tee -a "${level_dir}/apply.log"
    wait_for_workload_total "${namespace}" "${workloads}"
  fi

  workloads_time="$(curl_time_seconds "${KBEACON_URL}/api/v1/workloads?namespace=${namespace}&limit=1000")"
  dependency_map_time="$(curl_time_seconds "${KBEACON_URL}/api/v1/dependency-map?workloadNamespace=${namespace}&limit=1000")"
  secrets_time="$(curl_time_seconds "${KBEACON_URL}/api/v1/secrets?namespace=${namespace}&limit=1000")"
  config_time="$(curl_time_seconds "${KBEACON_URL}/api/v1/config")"

  curl_json "${KBEACON_URL}/api/v1/workloads?namespace=${namespace}&limit=1000" > "${level_dir}/workloads.json"
  curl_json "${KBEACON_URL}/api/v1/dependency-map?workloadNamespace=${namespace}&limit=1000" > "${level_dir}/dependency-map.json"
  curl_json "${KBEACON_URL}/api/v1/secrets?namespace=${namespace}&limit=1000" > "${level_dir}/secrets.json"
  curl_json "${KBEACON_URL}/api/v1/config" > "${level_dir}/config.json"

  observed_workloads="$(jq -r '.pagination.total // (.data | length)' "${level_dir}/workloads.json")"
  observed_edges="$(jq -r '.pagination.total // (.data.edges | length)' "${level_dir}/dependency-map.json")"
  observed_secrets="$(jq -r '.pagination.total // (.data | length)' "${level_dir}/secrets.json")"

  metrics_json="$(metrics_summary_json "${level_dir}/metrics.prom")"
  top_json="$(kubectl_top_json)"
  p95="$(prom_query_value 'histogram_quantile(0.95, sum by (le) (rate(kbeacon_graph_update_duration_seconds_bucket[5m])))')"

  jq -n \
    --arg generatedAt "$(now_iso)" \
    --arg namespace "${namespace}" \
    --argjson workloads "${workloads}" \
    --argjson secrets "${secrets}" \
    --argjson expectedEdges "${expected_edges}" \
    --argjson observedWorkloads "${observed_workloads}" \
    --argjson observedSecrets "${observed_secrets}" \
    --argjson observedEdges "${observed_edges}" \
    --argjson workloadsTime "${workloads_time}" \
    --argjson dependencyMapTime "${dependency_map_time}" \
    --argjson secretsTime "${secrets_time}" \
    --argjson configTime "${config_time}" \
    --argjson metrics "${metrics_json}" \
    --argjson kubectlTop "${top_json}" \
    --arg prometheusP95 "${p95}" \
    '{
      generatedAt: $generatedAt,
      namespace: $namespace,
      workloads: $workloads,
      secrets: $secrets,
      expectedEdges: $expectedEdges,
      observed: {
        workloads: $observedWorkloads,
        secrets: $observedSecrets,
        dependencyMapEdges: $observedEdges
      },
      apiSeconds: {
        config: $configTime,
        secretsList: $secretsTime,
        workloadsList: $workloadsTime,
        dependencyMap: $dependencyMapTime
      },
      metrics: $metrics,
      prometheusGraphRebuildP95Seconds: (if $prometheusP95 == "" then null else ($prometheusP95 | tonumber) end),
      kubectlTop: $kubectlTop
    }' > "${level_dir}/summary.json"

  cat "${level_dir}/summary.json" >> "${summary_levels_file}"

  if [[ "${RETAIN}" != "true" && "${APPLY}" == "true" ]]; then
    kubectl delete namespace "${namespace}" --ignore-not-found | tee "${level_dir}/cleanup.log"
  fi

done

jq -s \
  --arg generatedAt "$(now_iso)" \
  --arg kbeaconUrl "${KBEACON_URL}" \
  --arg prometheusUrl "${PROMETHEUS_URL}" \
  --arg runId "${RUN_ID}" \
  '{generatedAt:$generatedAt, runId:$runId, kbeaconUrl:$kbeaconUrl, prometheusUrl:$prometheusUrl, levels:.}' \
  "${summary_levels_file}" > "${RUN_DIR}/summary.json"

make_report_markdown "${RUN_DIR}/summary.json" "${RUN_DIR}/summary.md"

echo
echo "Scale benchmark complete."
echo "Summary JSON: ${RUN_DIR}/summary.json"
echo "Summary Markdown: ${RUN_DIR}/summary.md"
cat "${RUN_DIR}/summary.md"
