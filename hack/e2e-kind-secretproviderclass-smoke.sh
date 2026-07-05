#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "${ROOT_DIR}"

KIND="${KIND:-kind}"
KUBECTL="${KUBECTL:-kubectl}"
HELM="${HELM:-helm}"
DOCKER="${DOCKER:-docker}"
JQ="${JQ:-jq}"

CLUSTER_NAME="${KBEACON_SECRETPROVIDERCLASS_CLUSTER:-kbeacon-secretproviderclass-e2e}"
KBEACON_NAMESPACE="${KBEACON_NAMESPACE:-kbeacon-system}"
WORKLOAD_NAMESPACE="${KBEACON_SECRETPROVIDERCLASS_NAMESPACE:-kbeacon-secretproviderclass-e2e}"
RELEASE="${KBEACON_SECRETPROVIDERCLASS_RELEASE:-kbeacon-secretproviderclass}"
IMAGE_REPOSITORY="${KBEACON_SECRETPROVIDERCLASS_IMAGE_REPOSITORY:-kbeacon}"
IMAGE_TAG="${KBEACON_SECRETPROVIDERCLASS_IMAGE_TAG:-kind-secretproviderclass-e2e}"
CRD_NAME="secretproviderclasses.secrets-store.csi.x-k8s.io"

BASE_URL=""
PF_PID=""
PF_LOG="/tmp/kbeacon-kind-secretproviderclass-port-forward.log"
READY_JSON="/tmp/kbeacon-kind-secretproviderclass-ready.json"
IMPACT_JSON="/tmp/kbeacon-kind-secretproviderclass-impact.json"
TLS_IMPACT_JSON="/tmp/kbeacon-kind-secretproviderclass-tls-impact.json"
DEPS_JSON="/tmp/kbeacon-kind-secretproviderclass-dependencies.json"
MAP_JSON="/tmp/kbeacon-kind-secretproviderclass-map.json"
RBAC_YAML="/tmp/kbeacon-kind-secretproviderclass-rbac.yaml"
CRD_PREEXISTED=0
CREATED_CLUSTER=0

SOURCE_TYPE="secrets-store.csi.secretproviderclass.spec.secretObjects.secretName"

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
      "${KUBECTL}" delete crd "${CRD_NAME}" --ignore-not-found=true >/dev/null 2>&1 || true
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

for bin in "${KIND}" "${KUBECTL}" "${HELM}" "${DOCKER}" "${JQ}" curl python3; do
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

log "install minimal SecretProviderClass CRD"
if "${KUBECTL}" get crd "${CRD_NAME}" >/dev/null 2>&1; then
  CRD_PREEXISTED=1
  echo "SecretProviderClass CRD already exists; leaving it in place during cleanup"
else
  cat <<'YAML' | "${KUBECTL}" apply -f -
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: secretproviderclasses.secrets-store.csi.x-k8s.io
spec:
  group: secrets-store.csi.x-k8s.io
  names:
    kind: SecretProviderClass
    listKind: SecretProviderClassList
    plural: secretproviderclasses
    singular: secretproviderclass
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

"${KUBECTL}" wait --for=condition=Established "crd/${CRD_NAME}" --timeout=120s

log "prepare namespaces"
"${KUBECTL}" create namespace "${KBEACON_NAMESPACE}" --dry-run=client -o yaml | "${KUBECTL}" apply -f -
"${KUBECTL}" create namespace "${WORKLOAD_NAMESPACE}" --dry-run=client -o yaml | "${KUBECTL}" apply -f -

log "install KBeacon with SecretProviderClass watcher"
"${HELM}" uninstall "${RELEASE}" --namespace "${KBEACON_NAMESPACE}" --wait --timeout 60s >/dev/null 2>&1 || true

"${HELM}" upgrade --install "${RELEASE}" ./charts/kbeacon \
  --namespace "${KBEACON_NAMESPACE}" \
  --set cluster.name=kind-secretproviderclass \
  --set image.repository="${IMAGE_REPOSITORY}" \
  --set image.tag="${IMAGE_TAG}" \
  --set image.pullPolicy=IfNotPresent \
  --set dashboards.enabled=false \
  --set resourcesToWatch.secretsStore.secretProviderClasses=true \
  --wait \
  --timeout 180s

"${KUBECTL}" -n "${KBEACON_NAMESPACE}" rollout status deployment/"${RELEASE}" --timeout=180s
"${KUBECTL}" -n "${KBEACON_NAMESPACE}" get deploy,pods,svc -l app.kubernetes.io/instance="${RELEASE}"

log "verify SecretProviderClass RBAC rendered"
if "${KUBECTL}" get clusterrole "${RELEASE}" >/dev/null 2>&1; then
  "${KUBECTL}" get clusterrole "${RELEASE}" -o yaml | tee "${RBAC_YAML}"
else
  "${KUBECTL}" -n "${KBEACON_NAMESPACE}" get role "${RELEASE}" -o yaml | tee "${RBAC_YAML}"
fi

grep -Fq 'secrets-store.csi.x-k8s.io' "${RBAC_YAML}"
grep -Fq 'secretproviderclasses' "${RBAC_YAML}"

log "start port-forward"
PORT="${KBEACON_SECRETPROVIDERCLASS_PORT:-}"
if [ -z "${PORT}" ]; then
  PORT="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"
fi

BASE_URL="http://127.0.0.1:${PORT}"
rm -f "${PF_LOG}" "${READY_JSON}" "${IMPACT_JSON}" "${TLS_IMPACT_JSON}" "${DEPS_JSON}" "${MAP_JSON}"

"${KUBECTL}" -n "${KBEACON_NAMESPACE}" port-forward "svc/${RELEASE}" "${PORT}:8080" >"${PF_LOG}" 2>&1 &
PF_PID="$!"

READY=0
for _ in $(seq 1 60); do
  if curl -fsSL "${BASE_URL}/readyz" > "${READY_JSON}" 2>/dev/null; then
    if "${JQ}" -e '.status == "ready" and ([.caches[] | select(.resource == "SecretProviderClass" and .synced == true)] | length) == 1' "${READY_JSON}" >/dev/null; then
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
  echo "ERROR: KBeacon did not become ready with SecretProviderClass cache synced" >&2
  cat "${READY_JSON}" 2>/dev/null || true
  cat "${PF_LOG}" 2>/dev/null || true
  "${KUBECTL}" -n "${KBEACON_NAMESPACE}" logs deploy/"${RELEASE}" --all-containers --tail=200 || true
  exit 1
fi

cat "${READY_JSON}"

log "create SecretProviderClass object"
cat <<YAML | "${KUBECTL}" apply -f -
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: payments-vault
  namespace: ${WORKLOAD_NAMESPACE}
  annotations:
    kbeacon.io/discovery-mode: hybrid
    kbeacon.io/owner-team: platform
    kbeacon.io/criticality: high
  labels:
    app.kubernetes.io/name: payments-api
spec:
  provider: vault
  parameters:
    objects: |
      - objectName: /payments/db/password
        objectType: secret
      - objectName: /payments/tls/cert
        objectType: secret
  secretObjects:
    - secretName: payments-provider-secret
      type: Opaque
      data:
        - objectName: db-password
          key: password
    - secretName: payments-tls-secret
      type: kubernetes.io/tls
      data:
        - objectName: tls-crt
          key: tls.crt
YAML

"${KUBECTL}" -n "${WORKLOAD_NAMESPACE}" get secretproviderclasses -o yaml

wait_for_impact() {
  local secret_name="$1"
  local output="$2"

  local ready=0
  for _ in $(seq 1 60); do
    if curl -fsSL "${BASE_URL}/api/v1/secrets/${WORKLOAD_NAMESPACE}/${secret_name}/impact" > "${output}" 2>/dev/null; then
      if "${JQ}" -e '.data.summary.affectedWorkloadCount >= 1' "${output}" >/dev/null; then
        ready=1
        break
      fi
    fi
    sleep 2
  done

  if [ "${ready}" != "1" ]; then
    echo "ERROR: SecretProviderClass impact for ${secret_name} did not include expected affected object" >&2
    cat "${output}" 2>/dev/null || true
    "${KUBECTL}" -n "${KBEACON_NAMESPACE}" logs deploy/"${RELEASE}" --all-containers --tail=200 || true
    exit 1
  fi
}

log "wait for SecretProviderClass first secret impact"
wait_for_impact "payments-provider-secret" "${IMPACT_JSON}"
cat "${IMPACT_JSON}"

"${JQ}" -e --arg source_type "${SOURCE_TYPE}" '
  .data.secret.ref.name == "payments-provider-secret" and
  .data.secret.exists == false and
  .data.summary.affectedWorkloadCount >= 1 and
  ([
    .data.edges[]
    | select(
        .workload.kind == "SecretProviderClass" and
        .workload.name == "payments-vault" and
        (([.sources[]? | select(.type == $source_type and .path == "spec.secretObjects[0].secretName" and .resourceField == "spec.secretObjects[0].secretName")] | length) >= 1)
      )
  ] | length) >= 1
' "${IMPACT_JSON}" >/dev/null

echo "ok: SecretProviderClass first secret impact validated"

log "wait for SecretProviderClass second secret impact"
wait_for_impact "payments-tls-secret" "${TLS_IMPACT_JSON}"
cat "${TLS_IMPACT_JSON}"

"${JQ}" -e --arg source_type "${SOURCE_TYPE}" '
  .data.secret.ref.name == "payments-tls-secret" and
  .data.secret.exists == false and
  .data.summary.affectedWorkloadCount >= 1 and
  ([
    .data.edges[]
    | select(
        .workload.kind == "SecretProviderClass" and
        .workload.name == "payments-vault" and
        (([.sources[]? | select(.type == $source_type and .path == "spec.secretObjects[1].secretName" and .resourceField == "spec.secretObjects[1].secretName")] | length) >= 1)
      )
  ] | length) >= 1
' "${TLS_IMPACT_JSON}" >/dev/null

echo "ok: SecretProviderClass second secret impact validated"

log "verify SecretProviderClass workload dependencies"
curl -fsSL "${BASE_URL}/api/v1/workloads/${WORKLOAD_NAMESPACE}/SecretProviderClass/payments-vault/dependencies" > "${DEPS_JSON}"
cat "${DEPS_JSON}"

"${JQ}" -e --arg source_type "${SOURCE_TYPE}" '
  .data.workload.ref.kind == "SecretProviderClass" and
  ([
    .data.dependencies[]
    | select(
        .secret.ref.name == "payments-provider-secret" and
        .resolved == false and
        (([.sources[]? | select(.type == $source_type and .path == "spec.secretObjects[0].secretName")] | length) >= 1)
      )
  ] | length) >= 1 and
  ([
    .data.dependencies[]
    | select(
        .secret.ref.name == "payments-tls-secret" and
        .resolved == false and
        (([.sources[]? | select(.type == $source_type and .path == "spec.secretObjects[1].secretName")] | length) >= 1)
      )
  ] | length) >= 1
' "${DEPS_JSON}" >/dev/null

echo "ok: SecretProviderClass workload dependencies validated"

log "verify dependency map includes SecretProviderClass edges"
curl -fsSL "${BASE_URL}/api/v1/dependency-map" > "${MAP_JSON}"

"${JQ}" -e '
  ([
    .data.edges[]
    | select(.workload.kind == "SecretProviderClass" and .workload.name == "payments-vault" and .secret.name == "payments-provider-secret")
  ] | length) >= 1 and
  ([
    .data.edges[]
    | select(.workload.kind == "SecretProviderClass" and .workload.name == "payments-vault" and .secret.name == "payments-tls-secret")
  ] | length) >= 1
' "${MAP_JSON}" >/dev/null

echo "ok: dependency map includes SecretProviderClass edges"

log "agent logs"
"${KUBECTL}" -n "${KBEACON_NAMESPACE}" logs deploy/"${RELEASE}" --all-containers --tail=200

log "SecretProviderClass Kind smoke passed"
