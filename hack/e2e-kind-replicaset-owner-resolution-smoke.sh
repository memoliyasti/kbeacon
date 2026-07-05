#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

for tool in docker kind kubectl helm python3; do
  if ! command -v "${tool}" >/dev/null 2>&1; then
    echo "ERROR: ${tool} is required"
    exit 1
  fi
done

if ! docker info >/dev/null 2>&1; then
  echo "ERROR: docker daemon is not available"
  exit 1
fi

CLUSTER_NAME="${KBEACON_KIND_CLUSTER:-kbeacon-e2e-replicaset-owner-resolution}"
NAMESPACE="${KBEACON_E2E_NAMESPACE:-kbeacon-system}"
TEST_NAMESPACE="${KBEACON_E2E_TEST_NAMESPACE:-kbeacon-owner-resolution}"
IMAGE_REPOSITORY="${KBEACON_E2E_IMAGE_REPOSITORY:-kbeacon-agent}"
IMAGE_TAG="${KBEACON_E2E_IMAGE_TAG:-replicaset-owner-resolution-e2e}"
RELEASE_NAME="${KBEACON_E2E_RELEASE_NAME:-kbeacon}"

CREATED_CLUSTER="false"
PORT_FORWARD_PID=""
PORT_FORWARD_LOG="$(mktemp -t kbeacon-owner-resolution-port-forward.XXXXXX.log)"

cleanup() {
  if [ -n "${PORT_FORWARD_PID}" ]; then
    kill "${PORT_FORWARD_PID}" >/dev/null 2>&1 || true
  fi

  if [ "${CREATED_CLUSTER}" = "true" ] && [ "${KBEACON_KEEP_KIND_CLUSTER:-false}" != "true" ]; then
    kind delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if ! kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  echo "Creating Kind cluster ${CLUSTER_NAME}"
  kind create cluster --name "${CLUSTER_NAME}"
  CREATED_CLUSTER="true"
else
  echo "Using existing Kind cluster ${CLUSTER_NAME}"
fi

kind export kubeconfig --name "${CLUSTER_NAME}" >/dev/null

echo "Building image ${IMAGE_REPOSITORY}:${IMAGE_TAG}"
docker build -t "${IMAGE_REPOSITORY}:${IMAGE_TAG}" .

echo "Loading image into Kind"
kind load docker-image --name "${CLUSTER_NAME}" "${IMAGE_REPOSITORY}:${IMAGE_TAG}"

kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace "${TEST_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${TEST_NAMESPACE}" create secret generic owner-db \
  --from-literal=password=demo \
  --dry-run=client -o yaml | kubectl apply -f -

cat <<YAML | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: owner-api
  namespace: ${TEST_NAMESPACE}
  labels:
    app.kubernetes.io/name: owner-api
    app.kubernetes.io/team: owner-resolution
spec:
  replicas: 1
  selector:
    matchLabels:
      app: owner-api
  template:
    metadata:
      labels:
        app: owner-api
        app.kubernetes.io/team: owner-resolution
    spec:
      containers:
        - name: app
          image: busybox:1.36
          command: ["sh", "-c", "sleep 3600"]
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: owner-db
                  key: password
YAML

kubectl -n "${TEST_NAMESPACE}" rollout status deploy/owner-api --timeout=180s

install_kbeacon() {
  local replica_sets="$1"

  helm upgrade --install "${RELEASE_NAME}" ./charts/kbeacon \
    --namespace "${NAMESPACE}" \
    --create-namespace \
    --set cluster.name="${CLUSTER_NAME}" \
    --set image.repository="${IMAGE_REPOSITORY}" \
    --set image.tag="${IMAGE_TAG}" \
    --set image.pullPolicy=IfNotPresent \
    --set-string discovery.namespaces.include[0]="${TEST_NAMESPACE}" \
    --set resourcesToWatch.apps.replicaSets="${replica_sets}" \
    --wait \
    --timeout 240s

  kubectl -n "${NAMESPACE}" rollout status deploy/kbeacon --timeout=240s
}

PORT="$(
  python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"

start_port_forward() {
  if [ -n "${PORT_FORWARD_PID}" ]; then
    kill "${PORT_FORWARD_PID}" >/dev/null 2>&1 || true
    PORT_FORWARD_PID=""
  fi

  kubectl -n "${NAMESPACE}" port-forward svc/kbeacon "${PORT}:8080" >"${PORT_FORWARD_LOG}" 2>&1 &
  PORT_FORWARD_PID="$!"

  python3 - <<PY
import socket
import time

port = int("${PORT}")
deadline = time.time() + 45
last = None

while time.time() < deadline:
    try:
        with socket.create_connection(("127.0.0.1", port), timeout=1):
            raise SystemExit(0)
    except Exception as exc:
        last = exc
        time.sleep(1)

raise SystemExit(f"port-forward did not become ready: {last}")
PY
}

check_owner_resolution_enabled() {
  KBEACON_BASE_URL="http://127.0.0.1:${PORT}" \
  KBEACON_TEST_NAMESPACE="${TEST_NAMESPACE}" \
  python3 - <<'PY'
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

base_url = os.environ["KBEACON_BASE_URL"]
namespace = os.environ["KBEACON_TEST_NAMESPACE"]

def get_json(path):
    with urllib.request.urlopen(base_url + path, timeout=3) as response:
        return json.load(response)

def workload_key(item):
    ref = item.get("ref", {})
    return ref.get("kind"), ref.get("name")

def workload_deps(kind, name):
    path = "/api/v1/workloads/{}/{}/{}/dependencies".format(
        urllib.parse.quote(namespace, safe=""),
        urllib.parse.quote(kind, safe=""),
        urllib.parse.quote(name, safe=""),
    )
    return get_json(path).get("data", {})

def has_owner_db_dependency(payload):
    for dep in payload.get("dependencies", []):
        secret_ref = dep.get("secret", {}).get("ref", {})
        if (
            secret_ref.get("namespace") == namespace
            and secret_ref.get("name") == "owner-db"
            and dep.get("resolved") is True
        ):
            return True
    return False

deadline = time.time() + 180
last_error = None

while time.time() < deadline:
    try:
        ready = get_json("/readyz")
        caches = {item.get("resource"): item for item in ready.get("caches", [])}

        for required in ["Pod", "Deployment", "ReplicaSet", "Secret"]:
            cache = caches.get(required)
            if not cache or cache.get("synced") is not True:
                raise AssertionError(f"{required} cache not synced: {cache!r}")

        workloads = get_json(f"/api/v1/workloads?namespace={urllib.parse.quote(namespace, safe='')}").get("data", [])
        keys = {workload_key(item) for item in workloads}

        if ("Deployment", "owner-api") not in keys:
            raise AssertionError(f"missing Deployment workload: {workloads!r}")

        pod_workloads = [item for item in workloads if item.get("ref", {}).get("kind") == "Pod"]
        if pod_workloads:
            raise AssertionError(f"expected no Deployment-managed Pod duplicate when ReplicaSet cache is enabled: {pod_workloads!r}")

        deps = workload_deps("Deployment", "owner-api")
        if not has_owner_db_dependency(deps):
            raise AssertionError(f"missing resolved owner-db dependency from Deployment: {deps!r}")

        print(json.dumps({
            "mode": "replicaSets=true",
            "workloadCount": len(workloads),
            "podFallbackCount": len(pod_workloads),
            "deploymentDependencies": len(deps.get("dependencies", [])),
        }, sort_keys=True))
        sys.exit(0)
    except Exception as exc:
        last_error = exc
        time.sleep(3)

raise AssertionError(f"ReplicaSet-enabled owner resolution did not converge: {last_error}")
PY
}

check_owner_resolution_disabled_fallback() {
  KBEACON_BASE_URL="http://127.0.0.1:${PORT}" \
  KBEACON_TEST_NAMESPACE="${TEST_NAMESPACE}" \
  python3 - <<'PY'
import json
import os
import sys
import time
import urllib.parse
import urllib.request

base_url = os.environ["KBEACON_BASE_URL"]
namespace = os.environ["KBEACON_TEST_NAMESPACE"]

def get_json(path):
    with urllib.request.urlopen(base_url + path, timeout=3) as response:
        return json.load(response)

def workload_deps(kind, name):
    path = "/api/v1/workloads/{}/{}/{}/dependencies".format(
        urllib.parse.quote(namespace, safe=""),
        urllib.parse.quote(kind, safe=""),
        urllib.parse.quote(name, safe=""),
    )
    return get_json(path).get("data", {})

def has_owner_db_dependency(payload):
    for dep in payload.get("dependencies", []):
        secret_ref = dep.get("secret", {}).get("ref", {})
        if (
            secret_ref.get("namespace") == namespace
            and secret_ref.get("name") == "owner-db"
            and dep.get("resolved") is True
        ):
            return True
    return False

deadline = time.time() + 180
last_error = None

while time.time() < deadline:
    try:
        ready = get_json("/readyz")
        caches = {item.get("resource"): item for item in ready.get("caches", [])}

        replica_set_cache = caches.get("ReplicaSet")
        if not replica_set_cache or replica_set_cache.get("optional") is not True:
            raise AssertionError(f"expected ReplicaSet cache to be optional/disabled: {replica_set_cache!r}")

        workloads = get_json(f"/api/v1/workloads?namespace={urllib.parse.quote(namespace, safe='')}").get("data", [])

        deployments = [
            item for item in workloads
            if item.get("ref", {}).get("kind") == "Deployment"
            and item.get("ref", {}).get("name") == "owner-api"
        ]
        if not deployments:
            raise AssertionError(f"missing Deployment workload: {workloads!r}")

        pod_workloads = [
            item for item in workloads
            if item.get("ref", {}).get("kind") == "Pod"
            and item.get("ref", {}).get("name", "").startswith("owner-api-")
        ]
        if not pod_workloads:
            raise AssertionError(f"expected Pod fallback when ReplicaSet cache is disabled: {workloads!r}")

        pod_name = pod_workloads[0]["ref"]["name"]
        deps = workload_deps("Pod", pod_name)
        if not has_owner_db_dependency(deps):
            raise AssertionError(f"missing resolved owner-db dependency from Pod fallback {pod_name}: {deps!r}")

        print(json.dumps({
            "mode": "replicaSets=false",
            "workloadCount": len(workloads),
            "podFallback": pod_name,
            "podDependencies": len(deps.get("dependencies", [])),
        }, sort_keys=True))
        sys.exit(0)
    except Exception as exc:
        last_error = exc
        time.sleep(3)

raise AssertionError(f"ReplicaSet-disabled Pod fallback did not converge: {last_error}")
PY
}

echo "Installing KBeacon with ReplicaSet owner-resolution cache enabled"
install_kbeacon true
start_port_forward
check_owner_resolution_enabled

echo "Upgrading KBeacon with ReplicaSet owner-resolution cache disabled"
install_kbeacon false
start_port_forward
check_owner_resolution_disabled_fallback

echo "ok: ReplicaSet owner-resolution Kind smoke passed"
