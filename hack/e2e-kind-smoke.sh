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

echo
echo "===== kbeaconctl snapshot export smoke ====="

SNAPSHOT_NAMESPACE="${NAMESPACE:-kbeacon-system}"
SNAPSHOT_SERVICE="${KBEACON_SERVICE_NAME:-kbeacon}"

if ! kubectl -n "${SNAPSHOT_NAMESPACE}" get svc "${SNAPSHOT_SERVICE}" >/dev/null 2>&1
then
  SNAPSHOT_SERVICE="$(kubectl -n "${SNAPSHOT_NAMESPACE}" get svc -l app.kubernetes.io/name=kbeacon -o jsonpath='{.items[0].metadata.name}')"
fi

if [ -z "${SNAPSHOT_SERVICE}" ]; then
  echo "ERROR: could not resolve KBeacon Service name in namespace ${SNAPSHOT_NAMESPACE}"
  exit 1
fi

go build -trimpath -o ./bin/kbeaconctl ./cmd/kbeaconctl

SNAPSHOT_PORT="${KBEACON_SNAPSHOT_PORT:-18081}"

for candidate in 18081 18082 18083 18084 18085
do
  if ! (command -v lsof >/dev/null 2>&1 && lsof -iTCP:"${candidate}" -sTCP:LISTEN >/dev/null 2>&1)
  then
    SNAPSHOT_PORT="${candidate}"
    break
  fi
done

SNAPSHOT_URL="http://127.0.0.1:${SNAPSHOT_PORT}"
SNAPSHOT_FILE="/tmp/kbeacon-kind-snapshot.json"
SNAPSHOT_PF_LOG="/tmp/kbeacon-snapshot-port-forward.log"

rm -f "${SNAPSHOT_FILE}" "${SNAPSHOT_PF_LOG}"

kubectl -n "${SNAPSHOT_NAMESPACE}" port-forward "svc/${SNAPSHOT_SERVICE}" "${SNAPSHOT_PORT}:8080" >"${SNAPSHOT_PF_LOG}" 2>&1 &
SNAPSHOT_PF_PID="$!"

SNAPSHOT_READY=0

for _ in $(seq 1 30)
do
  if curl -fsSL "${SNAPSHOT_URL}/readyz" >/dev/null 2>&1
  then
    SNAPSHOT_READY=1
    break
  fi

  if ! kill -0 "${SNAPSHOT_PF_PID}" >/dev/null 2>&1
  then
    echo "ERROR: snapshot port-forward exited early"
    cat "${SNAPSHOT_PF_LOG}" || true
    exit 1
  fi

  sleep 1
done

if [ "${SNAPSHOT_READY}" != "1" ]; then
  echo "ERROR: KBeacon Agent was not reachable for snapshot export"
  cat "${SNAPSHOT_PF_LOG}" || true
  kill "${SNAPSHOT_PF_PID}" >/dev/null 2>&1 || true
  wait "${SNAPSHOT_PF_PID}" >/dev/null 2>&1 || true
  exit 1
fi

if ! ./bin/kbeaconctl \
  --server "${SNAPSHOT_URL}" \
  snapshot export \
  --include secrets,workloads,dependency-map,config \
  --output "${SNAPSHOT_FILE}"
then
  echo "ERROR: kbeaconctl snapshot export failed"
  cat "${SNAPSHOT_PF_LOG}" || true
  kill "${SNAPSHOT_PF_PID}" >/dev/null 2>&1 || true
  wait "${SNAPSHOT_PF_PID}" >/dev/null 2>&1 || true
  exit 1
fi

python3 -c '
import json
import sys

path = sys.argv[1]

with open(path, "r", encoding="utf-8") as f:
    snapshot = json.load(f)

if snapshot.get("kind") != "KBeaconSnapshot":
    raise SystemExit(f"expected kind=KBeaconSnapshot, got {snapshot.get(\"kind\")!r}")

resources = snapshot.get("resources")
if not isinstance(resources, dict):
    raise SystemExit("snapshot resources must be an object")

required_resources = ["config", "secrets", "workloads", "dependencyMap"]
missing = [name for name in required_resources if name not in resources]

if missing:
    raise SystemExit(f"snapshot is missing expected resources: {missing}")

for name in required_resources:
    if not isinstance(resources[name], dict):
        raise SystemExit(f"snapshot resource {name} must be an object")

config = resources["config"]
secrets = resources["secrets"]
workloads = resources["workloads"]
dependency_map = resources["dependencyMap"]

cluster = snapshot.get("cluster")
config_cluster = config.get("cluster") or config.get("data", {}).get("cluster", {}).get("name")

if not cluster:
    raise SystemExit("snapshot top-level cluster is empty")

if cluster != config_cluster:
    raise SystemExit(f"snapshot cluster {cluster!r} does not match config cluster {config_cluster!r}")

secret_count = len(secrets.get("data", []))
workload_count = len(workloads.get("data", []))
edge_count = len(dependency_map.get("data", {}).get("edges", []))

if secret_count <= 0:
    raise SystemExit("snapshot secrets resource is empty")

if workload_count <= 0:
    raise SystemExit("snapshot workloads resource is empty")

if edge_count <= 0:
    raise SystemExit("snapshot dependencyMap edges resource is empty")

print(f"snapshot export validation passed: cluster={cluster} secrets={secret_count} workloads={workload_count} edges={edge_count}")
' "${SNAPSHOT_FILE}"

kill "${SNAPSHOT_PF_PID}" >/dev/null 2>&1 || true
wait "${SNAPSHOT_PF_PID}" >/dev/null 2>&1 || true

ls -lh "${SNAPSHOT_FILE}"

echo
echo "===== kbeaconctl snapshot diff smoke ====="

SNAPSHOT_DIFF_TEXT="/tmp/kbeacon-kind-snapshot-diff.txt"
SNAPSHOT_DIFF_JSON="/tmp/kbeacon-kind-snapshot-diff.json"
SNAPSHOT_DIFF_FAIL_ON_CHANGE="/tmp/kbeacon-kind-snapshot-diff-fail-on-change.txt"

rm -f "${SNAPSHOT_DIFF_TEXT}" "${SNAPSHOT_DIFF_JSON}" "${SNAPSHOT_DIFF_FAIL_ON_CHANGE}"

./bin/kbeaconctl snapshot diff "${SNAPSHOT_FILE}" "${SNAPSHOT_FILE}" > "${SNAPSHOT_DIFF_TEXT}"

grep -q "KBeacon Snapshot Diff" "${SNAPSHOT_DIFF_TEXT}"

./bin/kbeaconctl snapshot diff --format json "${SNAPSHOT_FILE}" "${SNAPSHOT_FILE}" > "${SNAPSHOT_DIFF_JSON}"

python3 -c '
import json
import sys

path = sys.argv[1]

with open(path, "r", encoding="utf-8") as f:
    diff = json.load(f)

kind = diff.get("kind")
if kind != "KBeaconSnapshotDiff":
    raise SystemExit(f"expected kind=KBeaconSnapshotDiff, got {kind!r}")

serialized = json.dumps(diff, sort_keys=True)

for token in ("secrets", "workloads", "edges"):
    if token not in serialized:
        raise SystemExit(f"snapshot diff JSON missing expected token {token!r}")

print("snapshot diff JSON validation passed")
' "${SNAPSHOT_DIFF_JSON}"

./bin/kbeaconctl snapshot diff --fail-on-change "${SNAPSHOT_FILE}" "${SNAPSHOT_FILE}" > "${SNAPSHOT_DIFF_FAIL_ON_CHANGE}"

echo "snapshot diff smoke passed"

ls -lh "${SNAPSHOT_DIFF_TEXT}" "${SNAPSHOT_DIFF_JSON}" "${SNAPSHOT_DIFF_FAIL_ON_CHANGE}"


echo
echo "===== namespace-scoped low-privilege runtime smoke ====="

LOW_PRIV_NAMESPACE="kbeacon-lowpriv-e2e"
LOW_PRIV_RELEASE="kbeacon-lowpriv"
LOW_PRIV_PORT="${KBEACON_LOW_PRIV_PORT:-18086}"
LOW_PRIV_URL=""
LOW_PRIV_PF_LOG="/tmp/kbeacon-kind-lowpriv-port-forward.log"
LOW_PRIV_ROLE_YAML="/tmp/kbeacon-kind-lowpriv-role.yaml"
LOW_PRIV_IMPACT_JSON="/tmp/kbeacon-kind-lowpriv-impact.json"
LOW_PRIV_DEPENDENCIES_JSON="/tmp/kbeacon-kind-lowpriv-dependencies.json"
LOW_PRIV_SNAPSHOT_JSON="/tmp/kbeacon-kind-lowpriv-snapshot.json"

rm -f "${LOW_PRIV_PF_LOG}" "${LOW_PRIV_ROLE_YAML}" "${LOW_PRIV_IMPACT_JSON}" "${LOW_PRIV_DEPENDENCIES_JSON}" "${LOW_PRIV_SNAPSHOT_JSON}"
helm uninstall "${LOW_PRIV_RELEASE}" --namespace "${LOW_PRIV_NAMESPACE}" --wait --timeout 60s >/dev/null 2>&1 || true

kubectl create namespace "${LOW_PRIV_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${LOW_PRIV_NAMESPACE}" create secret generic app-db --from-literal=username=demo --from-literal=password=demo --dry-run=client -o yaml | kubectl apply -f -

if ! kubectl -n "${LOW_PRIV_NAMESPACE}" get deployment api >/dev/null 2>&1
then
  kubectl -n "${LOW_PRIV_NAMESPACE}" create deployment api --image=busybox:1.36 -- sh -c "sleep 3600"
fi

kubectl -n "${LOW_PRIV_NAMESPACE}" set env deployment/api --from=secret/app-db
kubectl -n "${LOW_PRIV_NAMESPACE}" annotate deployment/api kbeacon.io/discovery-mode=hybrid kbeacon.io/owner-team=platform kbeacon.io/criticality=high --overwrite
kubectl -n "${LOW_PRIV_NAMESPACE}" rollout status deployment/api --timeout=120s

MAIN_IMAGE="$(kubectl -n "${KBEACON_NAMESPACE:-kbeacon-system}" get deploy kbeacon -o jsonpath="{.spec.template.spec.containers[0].image}")"
echo "LOW_PRIV_MAIN_IMAGE=${MAIN_IMAGE}"

LOW_PRIV_IMAGE_ARGS=()
if [[ "${MAIN_IMAGE}" == *@* ]]
then
  LOW_PRIV_IMAGE_REPOSITORY="${MAIN_IMAGE%@*}"
  LOW_PRIV_IMAGE_DIGEST="${MAIN_IMAGE#*@}"
  LOW_PRIV_IMAGE_ARGS=(--set "image.repository=${LOW_PRIV_IMAGE_REPOSITORY}" --set "image.digest=${LOW_PRIV_IMAGE_DIGEST}" --set image.pullPolicy=IfNotPresent)
else
  LOW_PRIV_IMAGE_REPOSITORY="${MAIN_IMAGE%:*}"
  LOW_PRIV_IMAGE_TAG="${MAIN_IMAGE##*:}"

  if [ "${LOW_PRIV_IMAGE_REPOSITORY}" = "${MAIN_IMAGE}" ] || [ -z "${LOW_PRIV_IMAGE_TAG}" ]
  then
    echo "ERROR: could not parse image repository/tag from ${MAIN_IMAGE}"
    exit 1
  fi

  LOW_PRIV_IMAGE_ARGS=(--set "image.repository=${LOW_PRIV_IMAGE_REPOSITORY}" --set "image.tag=${LOW_PRIV_IMAGE_TAG}" --set image.pullPolicy=IfNotPresent)
fi

helm upgrade --install "${LOW_PRIV_RELEASE}" ./charts/kbeacon --namespace "${LOW_PRIV_NAMESPACE}" --set cluster.name=kind-lowpriv --set rbac.scope=namespace --set-string "discovery.namespaces.include[0]=${LOW_PRIV_NAMESPACE}" --set resourcesToWatch.core.secrets=false --set dashboards.enabled=false "${LOW_PRIV_IMAGE_ARGS[@]}" --wait --timeout 180s

kubectl -n "${LOW_PRIV_NAMESPACE}" rollout status deployment/"${LOW_PRIV_RELEASE}" --timeout=180s
kubectl -n "${LOW_PRIV_NAMESPACE}" get role,rolebinding,deploy,pods,svc -l app.kubernetes.io/instance="${LOW_PRIV_RELEASE}"

kubectl -n "${LOW_PRIV_NAMESPACE}" get role "${LOW_PRIV_RELEASE}" -o yaml > "${LOW_PRIV_ROLE_YAML}"
cat "${LOW_PRIV_ROLE_YAML}"

if grep -q "secrets" "${LOW_PRIV_ROLE_YAML}"
then
  echo "ERROR: namespace-scoped low-privilege Role unexpectedly contains Secret RBAC"
  exit 1
fi

echo "ok: namespace-scoped low-privilege Role has no Secret RBAC"

for candidate in 18086 18087 18088 18089 18090
do
  if ! (command -v lsof >/dev/null 2>&1 && lsof -iTCP:"${candidate}" -sTCP:LISTEN >/dev/null 2>&1)
  then
    LOW_PRIV_PORT="${candidate}"
    break
  fi
done

LOW_PRIV_URL="http://127.0.0.1:${LOW_PRIV_PORT}"

kubectl -n "${LOW_PRIV_NAMESPACE}" port-forward "svc/${LOW_PRIV_RELEASE}" "${LOW_PRIV_PORT}:8080" >"${LOW_PRIV_PF_LOG}" 2>&1 &
LOW_PRIV_PF_PID="$!"

LOW_PRIV_READY=0
for _ in $(seq 1 30)
do
  if curl -fsSL "${LOW_PRIV_URL}/readyz" > /tmp/kbeacon-kind-lowpriv-ready.json
  then
    LOW_PRIV_READY=1
    break
  fi

  if ! kill -0 "${LOW_PRIV_PF_PID}" >/dev/null 2>&1
  then
    echo "ERROR: low-privilege port-forward exited early"
    cat "${LOW_PRIV_PF_LOG}" || true
    exit 1
  fi

  sleep 1
done

if [ "${LOW_PRIV_READY}" != "1" ]
then
  echo "ERROR: low-privilege Agent was not reachable"
  cat "${LOW_PRIV_PF_LOG}" || true
  kubectl -n "${LOW_PRIV_NAMESPACE}" logs deploy/"${LOW_PRIV_RELEASE}" --tail=200 || true
  kill "${LOW_PRIV_PF_PID}" >/dev/null 2>&1 || true
  wait "${LOW_PRIV_PF_PID}" >/dev/null 2>&1 || true
  exit 1
fi

cat /tmp/kbeacon-kind-lowpriv-ready.json
python3 -c "import json,sys; data=json.load(open(sys.argv[1], \"r\", encoding=\"utf-8\")); assert data[\"status\"] == \"ready\", data" /tmp/kbeacon-kind-lowpriv-ready.json

kubectl -n "${LOW_PRIV_NAMESPACE}" logs deploy/"${LOW_PRIV_RELEASE}" --tail=200 > /tmp/kbeacon-kind-lowpriv-agent.log
cat /tmp/kbeacon-kind-lowpriv-agent.log

if grep -q "at the cluster scope" /tmp/kbeacon-kind-lowpriv-agent.log
then
  echo "ERROR: low-privilege Agent is still using cluster-scoped informer list/watch"
  kill "${LOW_PRIV_PF_PID}" >/dev/null 2>&1 || true
  wait "${LOW_PRIV_PF_PID}" >/dev/null 2>&1 || true
  exit 1
fi

./bin/kbeaconctl --server "${LOW_PRIV_URL}" ready

LOW_PRIV_IMPACT_READY=0
for _ in $(seq 1 60)
do
  if curl -fsSL "${LOW_PRIV_URL}/api/v1/secrets/${LOW_PRIV_NAMESPACE}/app-db/impact" > "${LOW_PRIV_IMPACT_JSON}"
  then
    if python3 -c "import json,sys; data=json.load(open(sys.argv[1], \"r\", encoding=\"utf-8\")); sys.exit(0 if data.get(\"data\", {}).get(\"summary\", {}).get(\"affectedWorkloadCount\", 0) >= 1 else 1)" "${LOW_PRIV_IMPACT_JSON}"
    then
      LOW_PRIV_IMPACT_READY=1
      break
    fi
  fi

  sleep 2
done

if [ "${LOW_PRIV_IMPACT_READY}" != "1" ]
then
  echo "ERROR: low-privilege impact graph did not include expected affected workload"
  cat "${LOW_PRIV_IMPACT_JSON}" 2>/dev/null || true
  kill "${LOW_PRIV_PF_PID}" >/dev/null 2>&1 || true
  wait "${LOW_PRIV_PF_PID}" >/dev/null 2>&1 || true
  exit 1
fi

cat "${LOW_PRIV_IMPACT_JSON}"
python3 -c "import json,sys; data=json.load(open(sys.argv[1], \"r\", encoding=\"utf-8\")); secret=data[\"data\"][\"secret\"]; summary=data[\"data\"][\"summary\"]; assert secret[\"exists\"] is False, secret; assert summary[\"affectedWorkloadCount\"] >= 1, summary; print(\"low-privilege impact validation passed\")" "${LOW_PRIV_IMPACT_JSON}"

curl -fsSL "${LOW_PRIV_URL}/api/v1/workloads/${LOW_PRIV_NAMESPACE}/Deployment/api/dependencies" > "${LOW_PRIV_DEPENDENCIES_JSON}"
cat "${LOW_PRIV_DEPENDENCIES_JSON}"
python3 -c "import json,sys; data=json.load(open(sys.argv[1], \"r\", encoding=\"utf-8\")); deps=data[\"data\"][\"dependencies\"]; matches=[d for d in deps if d[\"secret\"][\"ref\"][\"name\"] == \"app-db\" and d[\"resolved\"] is False]; assert matches, deps; print(\"low-privilege dependency validation passed\")" "${LOW_PRIV_DEPENDENCIES_JSON}"

./bin/kbeaconctl --server "${LOW_PRIV_URL}" snapshot export --output "${LOW_PRIV_SNAPSHOT_JSON}"
python3 -c "import json,sys; snapshot=json.load(open(sys.argv[1], \"r\", encoding=\"utf-8\")); resources=snapshot[\"resources\"]; assert snapshot[\"kind\"] == \"KBeaconSnapshot\", snapshot; assert snapshot.get(\"cluster\") == \"kind-lowpriv\", snapshot; assert len(resources[\"workloads\"][\"data\"]) > 0, resources[\"workloads\"]; assert len(resources[\"dependencyMap\"][\"data\"][\"edges\"]) > 0, resources[\"dependencyMap\"]; print(\"low-privilege snapshot validation passed\")" "${LOW_PRIV_SNAPSHOT_JSON}"

./bin/kbeaconctl snapshot diff --format markdown "${LOW_PRIV_SNAPSHOT_JSON}" "${LOW_PRIV_SNAPSHOT_JSON}" | sed -n "1,40p"

kill "${LOW_PRIV_PF_PID}" >/dev/null 2>&1 || true
wait "${LOW_PRIV_PF_PID}" >/dev/null 2>&1 || true
helm uninstall "${LOW_PRIV_RELEASE}" --namespace "${LOW_PRIV_NAMESPACE}" --wait --timeout 60s

echo "namespace-scoped low-privilege runtime smoke passed"
