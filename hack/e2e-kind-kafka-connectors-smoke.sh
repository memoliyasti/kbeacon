#!/usr/bin/env bash
set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

echo "===== Kafka connector Kind smoke ====="

if ! command -v kind >/dev/null 2>&1; then
  echo "ERROR: kind is required"
  exit 1
fi

if ! command -v kubectl >/dev/null 2>&1; then
  echo "ERROR: kubectl is required"
  exit 1
fi

if ! command -v helm >/dev/null 2>&1; then
  echo "ERROR: helm is required"
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: docker is required"
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "ERROR: python3 is required"
  exit 1
fi

CLUSTER_NAME="${KBEACON_KIND_CLUSTER_NAME:-}"
if [ -z "${CLUSTER_NAME}" ]; then
  existing_clusters="$(kind get clusters 2>/dev/null || true)"
  if printf '%s\n' "${existing_clusters}" | grep -qx "kbeacon-e2e"; then
    CLUSTER_NAME="kbeacon-e2e"
  elif [ "$(printf '%s\n' "${existing_clusters}" | sed '/^$/d' | wc -l | tr -d ' ')" = "1" ]; then
    CLUSTER_NAME="$(printf '%s\n' "${existing_clusters}" | sed '/^$/d' | head -n1)"
  else
    CLUSTER_NAME="kbeacon-kafka-connectors-smoke"
  fi
fi

NAMESPACE="${KBEACON_NAMESPACE:-kbeacon-system}"
TEST_NAMESPACE="${KBEACON_E2E_KAFKA_NAMESPACE:-payments}"
SHARED_NAMESPACE="${KBEACON_E2E_KAFKA_SHARED_NAMESPACE:-shared}"
PLATFORM_NAMESPACE="${KBEACON_E2E_KAFKA_PLATFORM_NAMESPACE:-platform}"

IMAGE_REPOSITORY="${KBEACON_E2E_IMAGE_REPOSITORY:-kbeacon-agent}"
IMAGE_TAG="${KBEACON_E2E_IMAGE_TAG:-kafka-connectors-$(git rev-parse --short HEAD)}"

CREATED_CLUSTER="false"
PORT_FORWARD_PID=""

cleanup() {
  if [ -n "${PORT_FORWARD_PID}" ]; then
    kill "${PORT_FORWARD_PID}" >/dev/null 2>&1 || true
    wait "${PORT_FORWARD_PID}" >/dev/null 2>&1 || true
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

echo
echo "===== build and load image ====="
docker build -t "${IMAGE_REPOSITORY}:${IMAGE_TAG}" .
kind load docker-image --name "${CLUSTER_NAME}" "${IMAGE_REPOSITORY}:${IMAGE_TAG}"

echo
echo "===== namespaces ====="
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace "${TEST_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace "${SHARED_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace "${PLATFORM_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

echo
echo "===== lightweight connector CRDs ====="
cat <<'YAML' | kubectl apply -f -
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: kafkaconnectors.kafka.strimzi.io
spec:
  group: kafka.strimzi.io
  scope: Namespaced
  names:
    plural: kafkaconnectors
    singular: kafkaconnector
    kind: KafkaConnector
    listKind: KafkaConnectorList
  versions:
    - name: v1beta2
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          x-kubernetes-preserve-unknown-fields: true
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: connectors.platform.confluent.io
spec:
  group: platform.confluent.io
  scope: Namespaced
  names:
    plural: connectors
    singular: connector
    kind: Connector
    listKind: ConnectorList
  versions:
    - name: v1beta1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          x-kubernetes-preserve-unknown-fields: true
YAML

kubectl wait --for=condition=Established crd/kafkaconnectors.kafka.strimzi.io --timeout=90s
kubectl wait --for=condition=Established crd/connectors.platform.confluent.io --timeout=90s

echo
echo "===== test Secrets ====="
kubectl -n "${TEST_NAMESPACE}" create secret generic mysql-connector-auth \
  --from-literal=password=mysql-password \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${TEST_NAMESPACE}" create secret generic jdbc-credentials \
  --from-literal=password=jdbc-password \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${TEST_NAMESPACE}" create secret generic kafka-auth \
  --from-literal=password=kafka-password \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${SHARED_NAMESPACE}" create secret generic kafka-auth \
  --from-literal=password=shared-kafka-password \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${PLATFORM_NAMESPACE}" create secret generic connect-rest-auth \
  --from-literal=username=connect \
  --from-literal=password=connect-password \
  --dry-run=client -o yaml | kubectl apply -f -

echo
echo "===== test connector resources ====="
cat <<YAML | kubectl apply -f -
apiVersion: kafka.strimzi.io/v1beta2
kind: KafkaConnector
metadata:
  name: mysql-source
  namespace: ${TEST_NAMESPACE}
  annotations:
    kbeacon.io/discovery-mode: hybrid
    kbeacon.io/owner-team: data-platform
    kbeacon.io/service: mysql-source
spec:
  class: io.debezium.connector.mysql.MySqlConnector
  tasksMax: 1
  config:
    database.hostname: mysql.${TEST_NAMESPACE}.svc.cluster.local
    database.user: app
    database.password: "\${secrets:mysql-connector-auth:password}"
    sasl.jaas.config: "org.apache.kafka.common.security.scram.ScramLoginModule required username='connector' password='\${secrets:${SHARED_NAMESPACE}/kafka-auth:password}';"
---
apiVersion: platform.confluent.io/v1beta1
kind: Connector
metadata:
  name: jdbc-sink
  namespace: ${TEST_NAMESPACE}
  annotations:
    kbeacon.io/discovery-mode: hybrid
    kbeacon.io/owner-team: data-platform
    kbeacon.io/service: jdbc-sink
spec:
  class: io.confluent.connect.jdbc.JdbcSinkConnector
  connectRest:
    authentication:
      basic:
        secretRef:
          namespace: ${PLATFORM_NAMESPACE}
          name: connect-rest-auth
  configs:
    connection.url: jdbc:postgresql://postgres.${TEST_NAMESPACE}.svc.cluster.local:5432/app
    connection.user: app
    connection.password: "\${file:/mnt/secrets/jdbc-credentials/password:password}"
    sasl.jaas.config: "org.apache.kafka.common.security.scram.ScramLoginModule required username='connector' password='\${file:/mnt/secrets/kafka-auth/password:password}';"
YAML

echo
echo "===== install / upgrade KBeacon ====="
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --set cluster.name="${CLUSTER_NAME}" \
  --set image.repository="${IMAGE_REPOSITORY}" \
  --set image.tag="${IMAGE_TAG}" \
  --set image.pullPolicy=IfNotPresent \
  --set resourcesToWatch.strimzi.kafkaConnectors=true \
  --set resourcesToWatch.confluent.connectors=true

kubectl -n "${NAMESPACE}" rollout status deploy/kbeacon --timeout=240s

PORT="$(python3 - <<'PY'
import socket
with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
    s.bind(("127.0.0.1", 0))
    print(s.getsockname()[1])
PY
)"

PORT_FORWARD_LOG="/tmp/kbeacon-kafka-connectors-port-forward-${PORT}.log"
kubectl -n "${NAMESPACE}" port-forward svc/kbeacon "${PORT}:8080" >"${PORT_FORWARD_LOG}" 2>&1 &
PORT_FORWARD_PID="$!"

python3 - <<PY
import json
import sys
import time
import urllib.error
import urllib.request

base = "http://127.0.0.1:${PORT}"
test_namespace = "${TEST_NAMESPACE}"
shared_namespace = "${SHARED_NAMESPACE}"
platform_namespace = "${PLATFORM_NAMESPACE}"

def get_json(path):
    with urllib.request.urlopen(base + path, timeout=5) as response:
        return json.loads(response.read().decode("utf-8"))

def wait_ready():
    last = None
    for _ in range(90):
        try:
            payload = get_json("/readyz")
            caches = payload.get("caches", [])
            by_name = {item.get("resource"): item for item in caches}

            kafka = by_name.get("KafkaConnector")
            confluent = by_name.get("Connector")

            if (
                payload.get("status") == "ready"
                and kafka
                and confluent
                and kafka.get("synced") is True
                and confluent.get("synced") is True
            ):
                return
            last = payload
        except Exception as exc:
            last = repr(exc)
        time.sleep(2)

    print("ERROR: KBeacon did not become ready with Kafka connector caches", file=sys.stderr)
    print(json.dumps(last, indent=2, sort_keys=True) if isinstance(last, dict) else last, file=sys.stderr)
    sys.exit(1)

def find_dependency(payload, namespace, name, source_type):
    data = payload.get("data", {})
    deps = data.get("dependencies", [])
    for dep in deps:
        secret = dep.get("secret", {}).get("ref", {})
        if secret.get("namespace") != namespace or secret.get("name") != name:
            continue

        sources = dep.get("sources", [])
        if any(source.get("type") == source_type for source in sources):
            return dep

    return None

def require_dependency(payload, namespace, name, source_type):
    dep = find_dependency(payload, namespace, name, source_type)
    if dep is None:
        raise AssertionError(f"missing dependency {namespace}/{name} source={source_type}")

    if dep.get("resolved") is not True:
        raise AssertionError(f"dependency {namespace}/{name} source={source_type} is not resolved: {dep!r}")

def wait_dependencies():
    last = None
    for _ in range(90):
        try:
            kafka = get_json(f"/api/v1/workloads/{test_namespace}/KafkaConnector/mysql-source/dependencies")
            confluent = get_json(f"/api/v1/workloads/{test_namespace}/Connector/jdbc-sink/dependencies")

            kafka_workload = kafka.get("data", {}).get("workload", {}).get("ref", {})
            confluent_workload = confluent.get("data", {}).get("workload", {}).get("ref", {})

            if kafka_workload.get("kind") != "KafkaConnector":
                raise AssertionError(f"unexpected KafkaConnector workload ref: {kafka_workload!r}")

            if confluent_workload.get("kind") != "Connector":
                raise AssertionError(f"unexpected Connector workload ref: {confluent_workload!r}")

            require_dependency(
                kafka,
                test_namespace,
                "mysql-connector-auth",
                "strimzi.kafkaconnector.spec.config.secrets",
            )
            require_dependency(
                kafka,
                shared_namespace,
                "kafka-auth",
                "strimzi.kafkaconnector.spec.config.secrets",
            )
            require_dependency(
                confluent,
                platform_namespace,
                "connect-rest-auth",
                "confluent.connector.spec.connectRest.authentication.secretRef",
            )
            require_dependency(
                confluent,
                test_namespace,
                "jdbc-credentials",
                "confluent.connector.spec.configs.file.mountedSecret",
            )
            require_dependency(
                confluent,
                test_namespace,
                "kafka-auth",
                "confluent.connector.spec.configs.file.mountedSecret",
            )

            print("ok: Kafka connector dependencies resolved")
            print(json.dumps({
                "kafkaConnectorDependencies": len(kafka.get("data", {}).get("dependencies", [])),
                "confluentConnectorDependencies": len(confluent.get("data", {}).get("dependencies", [])),
            }, indent=2, sort_keys=True))
            return
        except (urllib.error.HTTPError, urllib.error.URLError, AssertionError, TimeoutError) as exc:
            last = repr(exc)
            time.sleep(2)

    print("ERROR: connector dependencies were not observed", file=sys.stderr)
    print(last, file=sys.stderr)
    try:
        print(json.dumps(get_json("/api/v1/workloads"), indent=2, sort_keys=True), file=sys.stderr)
    except Exception:
        pass
    sys.exit(1)

wait_ready()
wait_dependencies()
PY

echo
echo "===== relevant workloads ====="
python3 - <<PY
import json
import urllib.request

base = "http://127.0.0.1:${PORT}"
for path in (
    "/api/v1/workloads?workloadKind=KafkaConnector",
    "/api/v1/workloads?workloadKind=Connector",
):
    with urllib.request.urlopen(base + path, timeout=5) as response:
        print(path)
        print(json.dumps(json.loads(response.read().decode("utf-8")), indent=2, sort_keys=True))
PY

echo
echo "ok: Kafka connector Kind smoke passed"
