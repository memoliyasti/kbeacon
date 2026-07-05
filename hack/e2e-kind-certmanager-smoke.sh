#!/usr/bin/env bash
set -Eeuo pipefail

KIND="${KIND:-kind}"
KUBECTL="${KUBECTL:-kubectl}"
HELM="${HELM:-helm}"
DOCKER="${DOCKER:-docker}"
JQ="${JQ:-jq}"

CLUSTER_NAME="${KBEACON_CERTMANAGER_CLUSTER:-kbeacon-certmanager-e2e}"
KBEACON_NAMESPACE="${KBEACON_NAMESPACE:-kbeacon-system}"
WORKLOAD_NAMESPACE="${KBEACON_CERTMANAGER_NAMESPACE:-kbeacon-certmanager-e2e}"
RELEASE="${KBEACON_CERTMANAGER_RELEASE:-kbeacon-certmanager}"
IMAGE_REPOSITORY="${KBEACON_CERTMANAGER_IMAGE_REPOSITORY:-kbeacon}"
IMAGE_TAG="${KBEACON_CERTMANAGER_IMAGE_TAG:-kind-certmanager-e2e}"
BASE_URL=""
PF_PID=""
PF_LOG="/tmp/kbeacon-kind-certmanager-port-forward.log"
READY_JSON="/tmp/kbeacon-kind-certmanager-ready.json"
IMPACT_JSON="/tmp/kbeacon-kind-certmanager-impact.json"
DEPS_JSON="/tmp/kbeacon-kind-certmanager-dependencies.json"
MAP_JSON="/tmp/kbeacon-kind-certmanager-map.json"
CRD_PREEXISTED=0
CREATED_CLUSTER=0

log() { printf '\n===== %s =====\n' "$1"; }

cleanup() {
  set +e
  if [ -n "${PF_PID}" ]; then
    kill "${PF_PID}" >/dev/null 2>&1 || true
    wait "${PF_PID}" >/dev/null 2>&1 || true
  fi

  if [ "${KBEACON_KEEP_KIND_CLUSTER:-0}" = "1" ]; then
    echo "keeping Kind cluster ${CLUSTER_NAME} because KBEACON_KEEP_KIND_CLUSTER=1"
    return 0
  fi

  if [ "${CREATED_CLUSTER}" = "1" ]; then
    "${KIND}" delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
  else
    "${HELM}" uninstall "${RELEASE}" --namespace "${KBEACON_NAMESPACE}" --wait --timeout 60s >/dev/null 2>&1 || true
    "${KUBECTL}" delete namespace "${WORKLOAD_NAMESPACE}" --ignore-not-found=true >/dev/null 2>&1 || true
    if [ "${CRD_PREEXISTED}" != "1" ]; then
      "${KUBECTL}" delete crd certificates.cert-manager.io --ignore-not-found=true >/dev/null 2>&1 || true
    fi
  fi
}
trap cleanup EXIT

require() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: required command not found: $1" >&2
    exit 1
  fi
}

for bin in "${KIND}" "${KUBECTL}" "${HELM}" "${DOCKER}" "${JQ}" python3; do
  require "${bin}"
done

log "ensure Kind cluster"
if "${KIND}" get clusters | grep -Fxq "${CLUSTER_NAME}"; then
  echo "using existing Kind cluster ${CLUSTER_NAME}"
else
  "${KIND}" create cluster --name "${CLUSTER_NAME}" --wait 120s
  CREATED_CLUSTER=1
fi

"${KUBECTL}" config use-context "kind-${CLUSTER_NAME}" >/dev/null
"${KUBECTL}" get nodes -o wide

log "build and load KBeacon image"
"${DOCKER}" build -t "${IMAGE_REPOSITORY}:${IMAGE_TAG}" .
"${KIND}" load docker-image "${IMAGE_REPOSITORY}:${IMAGE_TAG}" --name "${CLUSTER_NAME}"

log "install minimal cert-manager Certificate CRD"
if "${KUBECTL}" get crd certificates.cert-manager.io >/dev/null 2>&1; then
  CRD_PREEXISTED=1
  echo "cert-manager Certificate CRD already exists; leaving it in place during cleanup"
else
  cat <<'YAML' | "${KUBECTL}" apply -f -
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: certificates.cert-manager.io
spec:
  group: cert-manager.io
  names:
    categories:
      - cert-manager
    kind: Certificate
    listKind: CertificateList
    plural: certificates
    singular: certificate
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              x-kubernetes-preserve-unknown-fields: true
            status:
              type: object
              x-kubernetes-preserve-unknown-fields: true
YAML
fi
"${KUBECTL}" wait --for=condition=Established crd/certificates.cert-manager.io --timeout=120s

log "prepare namespaces"
"${KUBECTL}" create namespace "${KBEACON_NAMESPACE}" --dry-run=client -o yaml | "${KUBECTL}" apply -f -
"${KUBECTL}" create namespace "${WORKLOAD_NAMESPACE}" --dry-run=client -o yaml | "${KUBECTL}" apply -f -

log "install KBeacon with cert-manager Certificate watcher"
"${HELM}" uninstall "${RELEASE}" --namespace "${KBEACON_NAMESPACE}" --wait --timeout 60s >/dev/null 2>&1 || true
"${HELM}" upgrade --install "${RELEASE}" ./charts/kbeacon \
  --namespace "${KBEACON_NAMESPACE}" \
  --set cluster.name=kind-certmanager \
  --set image.repository="${IMAGE_REPOSITORY}" \
  --set image.tag="${IMAGE_TAG}" \
  --set image.pullPolicy=IfNotPresent \
  --set dashboards.enabled=false \
  --set resourcesToWatch.certManager.certificates=true \
  --wait \
  --timeout 180s

"${KUBECTL}" -n "${KBEACON_NAMESPACE}" rollout status deployment/"${RELEASE}" --timeout=180s
"${KUBECTL}" -n "${KBEACON_NAMESPACE}" get deploy,pods,svc -l app.kubernetes.io/instance="${RELEASE}"

log "verify cert-manager RBAC rendered"
if "${KUBECTL}" get clusterrole "${RELEASE}" >/dev/null 2>&1; then
  "${KUBECTL}" get clusterrole "${RELEASE}" -o yaml | tee /tmp/kbeacon-kind-certmanager-rbac.yaml
else
  "${KUBECTL}" -n "${KBEACON_NAMESPACE}" get role "${RELEASE}" -o yaml | tee /tmp/kbeacon-kind-certmanager-rbac.yaml
fi
grep -Fq 'cert-manager.io' /tmp/kbeacon-kind-certmanager-rbac.yaml
grep -Fq 'certificates' /tmp/kbeacon-kind-certmanager-rbac.yaml

log "start port-forward"
PORT="${KBEACON_CERTMANAGER_PORT:-}"
if [ -z "${PORT}" ]; then
  PORT="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(('127.0.0.1', 0))
print(s.getsockname()[1])
s.close()
PY
)"
fi
BASE_URL="http://127.0.0.1:${PORT}"
rm -f "${PF_LOG}" "${READY_JSON}" "${IMPACT_JSON}" "${DEPS_JSON}" "${MAP_JSON}"
"${KUBECTL}" -n "${KBEACON_NAMESPACE}" port-forward "svc/${RELEASE}" "${PORT}:8080" >"${PF_LOG}" 2>&1 &
PF_PID="$!"

READY=0
for _ in $(seq 1 60); do
  if curl -fsSL "${BASE_URL}/readyz" > "${READY_JSON}" 2>/dev/null; then
    if "${JQ}" -e '.status == "ready" and ([.caches[] | select(.resource == "Certificate" and .synced == true)] | length) == 1' "${READY_JSON}" >/dev/null; then
      READY=1
      break
    fi
  fi
  if ! kill -0 "${PF_PID}" >/dev/null 2>&1; then
    echo "ERROR: port-forward exited early" >&2
    cat "${PF_LOG}" >&2 || true
    exit 1
  fi
  sleep 2
done

if [ "${READY}" != "1" ]; then
  echo "ERROR: KBeacon did not become ready with Certificate cache synced" >&2
  cat "${READY_JSON}" 2>/dev/null || true
  cat "${PF_LOG}" 2>/dev/null || true
  "${KUBECTL}" -n "${KBEACON_NAMESPACE}" logs deploy/"${RELEASE}" --all-containers --tail=200 || true
  exit 1
fi
cat "${READY_JSON}"

log "create cert-manager Certificate object"
cat <<YAML | "${KUBECTL}" apply -f -
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: payments-tls
  namespace: ${WORKLOAD_NAMESPACE}
  annotations:
    kbeacon.io/discovery-mode: hybrid
    kbeacon.io/owner-team: platform
    kbeacon.io/criticality: high
  labels:
    app.kubernetes.io/name: payments-api
spec:
  secretName: payments-tls-secret
  dnsNames:
    - payments.example.com
  issuerRef:
    name: demo-issuer
    kind: ClusterIssuer
YAML
"${KUBECTL}" -n "${WORKLOAD_NAMESPACE}" get certificate payments-tls -o yaml

log "wait for Certificate target Secret impact"
IMPACT_READY=0
for _ in $(seq 1 60); do
  if curl -fsSL "${BASE_URL}/api/v1/secrets/${WORKLOAD_NAMESPACE}/payments-tls-secret/impact" > "${IMPACT_JSON}" 2>/dev/null; then
    if "${JQ}" -e '.data.summary.affectedWorkloadCount >= 1' "${IMPACT_JSON}" >/dev/null; then
      IMPACT_READY=1
      break
    fi
  fi
  sleep 2
done

if [ "${IMPACT_READY}" != "1" ]; then
  echo "ERROR: Certificate target Secret impact did not include expected affected object" >&2
  cat "${IMPACT_JSON}" 2>/dev/null || true
  "${KUBECTL}" -n "${KBEACON_NAMESPACE}" logs deploy/"${RELEASE}" --all-containers --tail=200 || true
  exit 1
fi
cat "${IMPACT_JSON}"
"${JQ}" -e '
  .data.secret.ref.name == "payments-tls-secret" and
  .data.secret.exists == false and
  .data.summary.affectedWorkloadCount >= 1 and
  ([.data.edges[] | select(.workload.kind == "Certificate" and .workload.name == "payments-tls" and (.sources[]?.type == "cert-manager.certificate.spec.secretName"))] | length) >= 1
' "${IMPACT_JSON}" >/dev/null
echo "ok: Certificate target Secret impact validated"

log "verify Certificate workload dependencies"
curl -fsSL "${BASE_URL}/api/v1/workloads/${WORKLOAD_NAMESPACE}/Certificate/payments-tls/dependencies" > "${DEPS_JSON}"
cat "${DEPS_JSON}"
"${JQ}" -e '
  .data.workload.ref.kind == "Certificate" and
  ([.data.dependencies[] | select(.secret.ref.name == "payments-tls-secret" and .resolved == false and (.sources[]?.type == "cert-manager.certificate.spec.secretName"))] | length) >= 1
' "${DEPS_JSON}" >/dev/null
echo "ok: Certificate workload dependency validated"

log "verify dependency map includes Certificate edge"
curl -fsSL "${BASE_URL}/api/v1/dependency-map" > "${MAP_JSON}"
"${JQ}" -e '
  ([.data.edges[] | select(.workload.kind == "Certificate" and .secret.name == "payments-tls-secret")] | length) >= 1
' "${MAP_JSON}" >/dev/null
echo "ok: dependency map includes Certificate edge"

log "agent logs"
"${KUBECTL}" -n "${KBEACON_NAMESPACE}" logs deploy/"${RELEASE}" --all-containers --tail=200

log "cert-manager Certificate Kind smoke passed"
