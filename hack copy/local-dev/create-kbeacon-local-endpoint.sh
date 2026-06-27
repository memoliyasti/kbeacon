#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-kube-addons}"
SERVICE_NAME="${SERVICE_NAME:-kbeacon-local}"
PORT="${PORT:-8080}"

HOST_IP="$(minikube ssh -- "getent hosts host.minikube.internal | awk '{print \$1}'" | tr -d '\r')"

if [[ -z "${HOST_IP}" ]]; then
  echo "failed to resolve host.minikube.internal from minikube"
  exit 1
fi

echo "Using host IP: ${HOST_IP}"

cat <<YAML | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: ${SERVICE_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: kbeacon
spec:
  ports:
    - name: http
      port: ${PORT}
      targetPort: ${PORT}
---
apiVersion: v1
kind: Endpoints
metadata:
  name: ${SERVICE_NAME}
  namespace: ${NAMESPACE}
subsets:
  - addresses:
      - ip: ${HOST_IP}
    ports:
      - name: http
        port: ${PORT}
YAML

kubectl -n "${NAMESPACE}" get svc "${SERVICE_NAME}"
kubectl -n "${NAMESPACE}" get endpoints "${SERVICE_NAME}" -o wide
