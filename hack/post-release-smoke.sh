#!/usr/bin/env bash
set -euo pipefail

TAG="${1:-}"
REPO="${REPO:-memoliyasti/kbeacon}"
HELM_REPO_NAME="${HELM_REPO_NAME:-kbeacon-release-smoke}"
HELM_REPO_URL="${HELM_REPO_URL:-https://memoliyasti.github.io/kbeacon/charts}"
CLUSTER_NAME="${CLUSTER_NAME:-kbeacon-release-smoke}"
NAMESPACE="${NAMESPACE:-kbeacon-system}"
MONITORING_NAMESPACE="${MONITORING_NAMESPACE:-monitoring}"

if [ -z "${TAG}" ]; then
  TAG="$(gh release view --repo "${REPO}" --json tagName --jq .tagName)"
fi

VERSION="${TAG#v}"

echo "TAG=${TAG}"
echo "VERSION=${VERSION}"

OS="$(uname -s | tr "[:upper:]" "[:lower:]")"
ARCH="$(uname -m)"
case "${ARCH}" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64) ARCH="amd64" ;;
  *) echo "unsupported arch: ${ARCH}"; exit 1 ;;
esac

WORKDIR="/tmp/kbeacon-release-smoke-${VERSION}"
rm -rf "${WORKDIR}"
mkdir -p "${WORKDIR}"

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
helm repo remove "${HELM_REPO_NAME}" 2>/dev/null || true
helm repo add "${HELM_REPO_NAME}" "${HELM_REPO_URL}"
helm repo update

helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
  --namespace "${MONITORING_NAMESPACE}" \
  --create-namespace \
  --set grafana.adminUser=admin \
  --set grafana.adminPassword=admin \
  --set grafana.sidecar.dashboards.enabled=true \
  --set grafana.sidecar.dashboards.searchNamespace=ALL \
  --set prometheus.prometheusSpec.retention=6h \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.ruleSelectorNilUsesHelmValues=false \
  --wait \
  --timeout 15m

gh release download "${TAG}" --repo "${REPO}" --pattern "kbeacon_${TAG}_${OS}_${ARCH}" --dir "${WORKDIR}" --clobber
chmod +x "${WORKDIR}/kbeacon_${TAG}_${OS}_${ARCH}"
KBEACON="${WORKDIR}/kbeacon_${TAG}_${OS}_${ARCH}"
${KBEACON} version

helm upgrade --install kbeacon "${HELM_REPO_NAME}/kbeacon" \
  --version "${VERSION}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --set cluster.name="${CLUSTER_NAME}" \
  --set dashboards.enabled=true \
  --set serviceMonitor.enabled=true \
  --set prometheus.scrapeAnnotations.enabled=true \
  --wait \
  --timeout 10m

kubectl -n "${NAMESPACE}" rollout status deploy/kbeacon --timeout=5m
${KBEACON} --namespace "${NAMESPACE}" ready | jq -e ".status == \"ready\""
${KBEACON} --namespace "${NAMESPACE}" get config | jq .

if [ -d examples/demo-blast-radius ]; then
  kubectl apply -f examples/demo-blast-radius/namespace.yaml
  kubectl apply -f examples/demo-blast-radius/secrets.yaml
  kubectl apply -f examples/demo-blast-radius/workloads.yaml
  sleep 45
  ${KBEACON} --namespace "${NAMESPACE}" get secrets --limit 200 > "${WORKDIR}/secrets.json"
  ${KBEACON} --namespace "${NAMESPACE}" get workloads --limit 200 > "${WORKDIR}/workloads.json"
  ${KBEACON} --namespace "${NAMESPACE}" get dependency-map --limit 500 > "${WORKDIR}/dependency-map.json"
  ${KBEACON} --namespace "${NAMESPACE}" impact --format json payments payments-db > "${WORKDIR}/impact.json"
  ${KBEACON} --namespace "${NAMESPACE}" snapshot export --output "${WORKDIR}/snapshot.json"
  grep -RniE "demo-password-should-not-leak|demo-key-should-not-leak|connect-token-should-not-leak|provider-token-should-not-leak" "${WORKDIR}" && exit 1 || true
  jq -e ".data.summary.affectedWorkloadCount >= 1" "${WORKDIR}/impact.json"
  test -s "${WORKDIR}/snapshot.json"
fi

PROM_SVC="$(kubectl -n "${MONITORING_NAMESPACE}" get svc -o name | grep -E "prometheus.*prometheus|kube-prometheus.*prometheus" | head -1)"
test -n "${PROM_SVC}"
kubectl -n "${MONITORING_NAMESPACE}" port-forward "${PROM_SVC}" 9090:9090 >"${WORKDIR}/prometheus-port-forward.log" 2>&1 &
PROM_PID="$!"
sleep 6
for i in $(seq 1 30); do
  if curl -fsS --get http://127.0.0.1:9090/api/v1/query --data-urlencode "query=up{namespace=\"kbeacon-system\"}" | jq -e ".data.result | length >= 1" >/dev/null; then
    break
  fi
  sleep 5
done
curl -fsS --get http://127.0.0.1:9090/api/v1/query --data-urlencode "query=up{namespace=\"kbeacon-system\"}" | jq -e ".data.result | length >= 1"
curl -fsS --get http://127.0.0.1:9090/api/v1/query --data-urlencode "query=kbeacon_cluster_dependency_count" | jq -e ".data.result | length >= 1"
kill "${PROM_PID}" 2>/dev/null || true

GRAFANA_ADMIN_USER="${GRAFANA_ADMIN_USER:-admin}"
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-admin}"

GRAFANA_SVC="$(kubectl -n "${MONITORING_NAMESPACE}" get svc -o name | grep grafana | head -1)"
test -n "${GRAFANA_SVC}"
kubectl -n "${MONITORING_NAMESPACE}" port-forward "${GRAFANA_SVC}" 3000:80 >"${WORKDIR}/grafana-port-forward.log" 2>&1 &
GRAFANA_PID="$!"
sleep 8
curl -fsS --user "${GRAFANA_ADMIN_USER}:${GRAFANA_ADMIN_PASSWORD}" http://127.0.0.1:3000/api/health | jq .
for i in $(seq 1 30); do
  if curl -fsS --user "${GRAFANA_ADMIN_USER}:${GRAFANA_ADMIN_PASSWORD}" "http://127.0.0.1:3000/api/search?query=KBeacon" | jq -e "length >= 1" >/dev/null; then
    break
  fi
  sleep 5
done
curl -fsS --user "${GRAFANA_ADMIN_USER}:${GRAFANA_ADMIN_PASSWORD}" "http://127.0.0.1:3000/api/search?query=KBeacon" | jq -e "length >= 1"
kill "${GRAFANA_PID}" 2>/dev/null || true

echo "OK: public Helm install, kube-native CLI, Prometheus target, Grafana dashboards, demo impact, snapshot, and no-secret-leak smoke passed"
