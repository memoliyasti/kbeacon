#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "${ROOT_DIR}"

KIND="${KIND:-kind}"
KUBECTL="${KUBECTL:-kubectl}"
HELM="${HELM:-helm}"
DOCKER="${DOCKER:-docker}"
PYTHON="${PYTHON:-python3}"

if [ -n "${KBEACON_E2E_CLUSTER:-}" ]; then
  CLUSTER_NAME="${KBEACON_E2E_CLUSTER}"
else
  CLUSTER_NAME="kbeacon-e2e-$(date +%s)"
fi

KBEACON_NAMESPACE="${KBEACON_E2E_NAMESPACE:-kbeacon-system}"
WORKLOAD_NAMESPACE="${KBEACON_E2E_WORKLOAD_NAMESPACE:-kbeacon-e2e}"
IMAGE_REPOSITORY="${KBEACON_E2E_IMAGE_REPOSITORY:-kbeacon-agent}"
IMAGE_TAG="${KBEACON_E2E_IMAGE_TAG:-e2e}"
PORT="${KBEACON_E2E_PORT:-18080}"
KEEP_CLUSTER="${KBEACON_E2E_KEEP_CLUSTER:-0}"
PORT_FORWARD_PID=""

require_command() {
  local name="$1"
  if ! command -v "${name}" >/dev/null 2>&1; then
    echo "ERROR: required command not found: ${name}"
    exit 1
  fi
}

cleanup() {
  local code=$?
  set +e

  if [ -n "${PORT_FORWARD_PID}" ] && kill -0 "${PORT_FORWARD_PID}" >/dev/null 2>&1; then
    kill "${PORT_FORWARD_PID}" >/dev/null 2>&1 || true
    wait "${PORT_FORWARD_PID}" >/dev/null 2>&1 || true
  fi

  if [ "${KEEP_CLUSTER}" != "1" ]; then
    "${KIND}" delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
  else
    echo "keeping Kind cluster: ${CLUSTER_NAME}"
  fi

  exit "${code}"
}

on_error() {
  local line="$1"
  set +e
  echo
  echo "ERROR: Kind E2E smoke failed at line ${line}"
  echo
  echo "===== debug: pods ====="
  "${KUBECTL}" get pods -A -o wide || true
  echo
  echo "===== debug: kbeacon logs ====="
  "${KUBECTL}" -n "${KBEACON_NAMESPACE}" logs deploy/kbeacon --tail=200 || true
  echo
  echo "===== debug: port-forward log ====="
  cat /tmp/kbeacon-kind-e2e-port-forward.log 2>/dev/null || true
}

trap cleanup EXIT
trap 'on_error ${LINENO}' ERR

require_command "${KIND}"
require_command "${KUBECTL}"
require_command "${HELM}"
require_command "${DOCKER}"
require_command "${PYTHON}"

echo "cluster=${CLUSTER_NAME}"
echo "kbeacon_namespace=${KBEACON_NAMESPACE}"
echo "workload_namespace=${WORKLOAD_NAMESPACE}"
echo "image=${IMAGE_REPOSITORY}:${IMAGE_TAG}"

if "${KIND}" get clusters | grep -Fxq "${CLUSTER_NAME}"; then
  echo "existing Kind cluster found; deleting: ${CLUSTER_NAME}"
  "${KIND}" delete cluster --name "${CLUSTER_NAME}"
fi

echo
echo "===== build local image ====="
"${DOCKER}" build -t "${IMAGE_REPOSITORY}:${IMAGE_TAG}" .

echo
echo "===== create Kind cluster ====="
create_args=(create cluster --name "${CLUSTER_NAME}" --wait 120s)
if [ -n "${KIND_NODE_IMAGE:-}" ]; then
  create_args+=(--image "${KIND_NODE_IMAGE}")
fi
"${KIND}" "${create_args[@]}"

"${KUBECTL}" config use-context "kind-${CLUSTER_NAME}" >/dev/null

echo
echo "===== load image into Kind ====="
"${KIND}" load docker-image "${IMAGE_REPOSITORY}:${IMAGE_TAG}" --name "${CLUSTER_NAME}"

echo
echo "===== install KBeacon chart ====="
"${HELM}" upgrade --install kbeacon ./charts/kbeacon \
  --namespace "${KBEACON_NAMESPACE}" \
  --create-namespace \
  --set cluster.name=kind-e2e \
  --set image.repository="${IMAGE_REPOSITORY}" \
  --set image.tag="${IMAGE_TAG}" \
  --set image.pullPolicy=IfNotPresent \
  --set privacy.redaction.secretKeys=true \
  --wait \
  --timeout 180s

"${KUBECTL}" -n "${KBEACON_NAMESPACE}" rollout status deploy/kbeacon --timeout=180s

echo
echo "===== apply workload fixture ====="
"${KUBECTL}" create namespace "${WORKLOAD_NAMESPACE}" --dry-run=client -o yaml | "${KUBECTL}" apply -f -

cat <<'YAML' | "${KUBECTL}" apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: key-secret
  namespace: kbeacon-e2e
type: Opaque
stringData:
  password: demo
---
apiVersion: v1
kind: Secret
metadata:
  name: envfrom-secret
  namespace: kbeacon-e2e
type: Opaque
stringData:
  config: demo
---
apiVersion: v1
kind: Secret
metadata:
  name: volume-secret
  namespace: kbeacon-e2e
type: Opaque
stringData:
  file: demo
---
apiVersion: v1
kind: Secret
metadata:
  name: projected-secret
  namespace: kbeacon-e2e
type: Opaque
stringData:
  token: demo
---
apiVersion: v1
kind: Secret
metadata:
  name: explicit-secret
  namespace: kbeacon-e2e
type: Opaque
stringData:
  value: demo
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2e-api
  namespace: kbeacon-e2e
  annotations:
    kbeacon.io/discovery-mode: hybrid
    kbeacon.io/watch-secrets: explicit-secret
    kbeacon.io/owner-team: platform-e2e
    kbeacon.io/criticality: critical
spec:
  replicas: 1
  selector:
    matchLabels:
      app: e2e-api
  template:
    metadata:
      labels:
        app: e2e-api
    spec:
      containers:
        - name: app
          image: busybox:1.36
          command:
            - sh
            - -c
            - sleep 3600
          env:
            - name: APP_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: key-secret
                  key: password
          envFrom:
            - secretRef:
                name: envfrom-secret
          volumeMounts:
            - name: secret-volume
              mountPath: /var/run/e2e/secret
              readOnly: true
            - name: projected-config
              mountPath: /var/run/e2e/projected
              readOnly: true
      volumes:
        - name: secret-volume
          secret:
            secretName: volume-secret
        - name: projected-config
          projected:
            sources:
              - secret:
                  name: projected-secret
                  optional: true
                  items:
                    - key: token
                      path: token
YAML

echo
echo "===== start port-forward ====="
rm -f /tmp/kbeacon-kind-e2e-port-forward.log
"${KUBECTL}" -n "${KBEACON_NAMESPACE}" port-forward svc/kbeacon "${PORT}:8080" >/tmp/kbeacon-kind-e2e-port-forward.log 2>&1 &
PORT_FORWARD_PID="$!"

export KBEACON_E2E_BASE_URL="http://127.0.0.1:${PORT}"
export KBEACON_E2E_WORKLOAD_NAMESPACE="${WORKLOAD_NAMESPACE}"

echo
echo "===== verify Agent API ====="
"${PYTHON}" <<'PY'
import json
import os
import sys
import time
import urllib.request

base_url = os.environ["KBEACON_E2E_BASE_URL"].rstrip("/")
workload_namespace = os.environ["KBEACON_E2E_WORKLOAD_NAMESPACE"]

def get_json(path):
    with urllib.request.urlopen(base_url + path, timeout=5) as response:
        return json.loads(response.read().decode("utf-8"))

deadline = time.time() + 120
last_error = None

while time.time() < deadline:
    try:
        ready = get_json("/readyz")
        if ready.get("status") == "ready":
            print("ok: readyz is ready")
            break
    except Exception as exc:
        last_error = exc
    time.sleep(2)
else:
    raise SystemExit(f"readyz did not become ready: {last_error}")

expected_secrets = {
    "key-secret",
    "envfrom-secret",
    "volume-secret",
    "projected-secret",
    "explicit-secret",
}

deadline = time.time() + 120
last_seen = []

while time.time() < deadline:
    dep_map = get_json(f"/api/v1/dependency-map?namespace={workload_namespace}&limit=100")
    edges = dep_map.get("data", {}).get("edges", [])
    edges = [
        edge for edge in edges
        if edge.get("workload", {}).get("namespace") == workload_namespace
    ]

    seen = {
        edge.get("secret", {}).get("name")
        for edge in edges
    }
    last_seen = sorted(name for name in seen if name)

    if expected_secrets.issubset(seen):
        break

    time.sleep(2)
else:
    missing = sorted(expected_secrets.difference(last_seen))
    raise SystemExit(f"missing expected dependency edges: {missing}; last_seen={last_seen}")

projected_sources = []
redaction_paths = []

for edge in edges:
    secret_name = edge.get("secret", {}).get("name")
    for source in edge.get("sources", []):
        if secret_name == "projected-secret":
            projected_sources.append(source)
        if secret_name == "key-secret":
            redaction_paths.append(source.get("path", ""))

if not any(source.get("type") == "volumes.projected.sources.secret" for source in projected_sources):
    raise SystemExit(f"projected Secret source type not found: {projected_sources}")

if not any("<redacted>" in path for path in redaction_paths):
    raise SystemExit(f"redacted Secret key path not found: {redaction_paths}")

if any("#password" in path for path in redaction_paths):
    raise SystemExit(f"Secret key leaked in source path: {redaction_paths}")

print("ok: expected Secret dependency edges found")
print("ok: projected Secret volume source verified")
print("ok: Secret key redaction verified")
PY

echo
echo "ok: Kind E2E smoke test completed"
