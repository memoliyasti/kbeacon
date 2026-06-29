#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${1:-/tmp/kbeacon-scale-fixture}"
NAMESPACE="${2:-kbeacon-scale}"
SECRET_COUNT="${3:-50}"
WORKLOAD_COUNT="${4:-200}"

if [ "${SECRET_COUNT}" -lt 3 ]; then
  echo "SECRET_COUNT must be at least 3"
  exit 1
fi

if [ "${WORKLOAD_COUNT}" -lt 1 ]; then
  echo "WORKLOAD_COUNT must be at least 1"
  exit 1
fi

mkdir -p "${OUT_DIR}"

pad() {
  printf "%03d" "$1"
}

criticality_for() {
  case $(("$1" % 4)) in
    0) printf "%s" "low" ;;
    1) printf "%s" "medium" ;;
    2) printf "%s" "high" ;;
    *) printf "%s" "critical" ;;
  esac
}

printf "%s\n" "apiVersion: v1" "kind: Namespace" "metadata:" "  name: ${NAMESPACE}" > "${OUT_DIR}/namespace.yaml"

: > "${OUT_DIR}/secrets.yaml"
for i in $(seq 0 $((SECRET_COUNT - 1))); do
  id="$(pad "${i}")"
  team="team-$((i % 10))"
  criticality="$(criticality_for "${i}")"
  {
    printf "%s\n" "---"
    printf "%s\n" "apiVersion: v1"
    printf "%s\n" "kind: Secret"
    printf "%s\n" "metadata:"
    printf "%s\n" "  name: scale-secret-${id}"
    printf "%s\n" "  namespace: ${NAMESPACE}"
    printf "%s\n" "  annotations:"
    printf "%s\n" "    kbeacon.io/owner-team: ${team}"
    printf "%s\n" "    kbeacon.io/criticality: ${criticality}"
    printf "%s\n" "type: Opaque"
    printf "%s\n" "stringData:"
    printf "%s\n" "  token: demo-${id}"
  } >> "${OUT_DIR}/secrets.yaml"
done

: > "${OUT_DIR}/workloads.yaml"
for i in $(seq 0 $((WORKLOAD_COUNT - 1))); do
  id="$(pad "${i}")"
  s0="$(pad $((i % SECRET_COUNT)))"
  s1="$(pad $(((i + 1) % SECRET_COUNT)))"
  s2="$(pad $(((i + 2) % SECRET_COUNT)))"
  team="team-$((i % 10))"
  criticality="$(criticality_for "${i}")"
  {
    printf "%s\n" "---"
    printf "%s\n" "apiVersion: apps/v1"
    printf "%s\n" "kind: Deployment"
    printf "%s\n" "metadata:"
    printf "%s\n" "  name: scale-app-${id}"
    printf "%s\n" "  namespace: ${NAMESPACE}"
    printf "%s\n" "  annotations:"
    printf "%s\n" "    kbeacon.io/enabled: \"true\""
    printf "%s\n" "    kbeacon.io/discovery-mode: hybrid"
    printf "%s\n" "    kbeacon.io/owner-team: ${team}"
    printf "%s\n" "    kbeacon.io/criticality: ${criticality}"
    printf "%s\n" "spec:"
    printf "%s\n" "  replicas: 1"
    printf "%s\n" "  selector:"
    printf "%s\n" "    matchLabels:"
    printf "%s\n" "      app: scale-app-${id}"
    printf "%s\n" "  template:"
    printf "%s\n" "    metadata:"
    printf "%s\n" "      labels:"
    printf "%s\n" "        app: scale-app-${id}"
    printf "%s\n" "    spec:"
    printf "%s\n" "      containers:"
    printf "%s\n" "        - name: app"
    printf "%s\n" "          image: busybox:1.36"
    printf "%s\n" "          command: [\"sh\", \"-c\", \"sleep 3600\"]"
    printf "%s\n" "          env:"
    printf "%s\n" "            - name: SCALE_TOKEN"
    printf "%s\n" "              valueFrom:"
    printf "%s\n" "                secretKeyRef:"
    printf "%s\n" "                  name: scale-secret-${s0}"
    printf "%s\n" "                  key: token"
    printf "%s\n" "          envFrom:"
    printf "%s\n" "            - secretRef:"
    printf "%s\n" "                name: scale-secret-${s1}"
    printf "%s\n" "          volumeMounts:"
    printf "%s\n" "            - name: scale-secret-volume"
    printf "%s\n" "              mountPath: /var/run/secrets/scale"
    printf "%s\n" "              readOnly: true"
    printf "%s\n" "      volumes:"
    printf "%s\n" "        - name: scale-secret-volume"
    printf "%s\n" "          secret:"
    printf "%s\n" "            secretName: scale-secret-${s2}"
  } >> "${OUT_DIR}/workloads.yaml"
done

expected_edges=$((WORKLOAD_COUNT * 3))
printf "%s\n" "{" > "${OUT_DIR}/expected-summary.json"
printf "%s\n" "  \"namespace\": \"${NAMESPACE}\"," >> "${OUT_DIR}/expected-summary.json"
printf "%s\n" "  \"secretCount\": ${SECRET_COUNT}," >> "${OUT_DIR}/expected-summary.json"
printf "%s\n" "  \"workloadCount\": ${WORKLOAD_COUNT}," >> "${OUT_DIR}/expected-summary.json"
printf "%s\n" "  \"expectedDependencyEdges\": ${expected_edges}," >> "${OUT_DIR}/expected-summary.json"
printf "%s\n" "  \"edgesPerWorkload\": 3" >> "${OUT_DIR}/expected-summary.json"
printf "%s\n" "}" >> "${OUT_DIR}/expected-summary.json"

printf "%s\n" "# KBeacon scale fixture" > "${OUT_DIR}/README.md"
printf "%s\n" "" >> "${OUT_DIR}/README.md"
printf "%s\n" "Generated namespace: ${NAMESPACE}" >> "${OUT_DIR}/README.md"
printf "%s\n" "Generated Secrets: ${SECRET_COUNT}" >> "${OUT_DIR}/README.md"
printf "%s\n" "Generated Deployments: ${WORKLOAD_COUNT}" >> "${OUT_DIR}/README.md"
printf "%s\n" "Expected dependency edges: ${expected_edges}" >> "${OUT_DIR}/README.md"

echo "Generated scale fixture in ${OUT_DIR}"
echo "namespace=${NAMESPACE}"
echo "secrets=${SECRET_COUNT}"
echo "workloads=${WORKLOAD_COUNT}"
echo "expected_edges=${expected_edges}"
