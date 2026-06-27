#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-kbeacon-system}"
IMAGE="${IMAGE:-kbeacon-agent}"
TAG="${TAG:-dev}"

kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

echo "Using Minikube Docker daemon"
eval "$(minikube docker-env)"

echo "Building ${IMAGE}:${TAG}"
docker build -t "${IMAGE}:${TAG}" .

echo "Installing KBeacon into namespace ${NAMESPACE}"
helm upgrade --install kbeacon ./charts/kbeacon \
  --namespace "${NAMESPACE}" \
  --values hack/local-dev/kbeacon-minikube-values.yaml

kubectl -n "${NAMESPACE}" rollout status deploy/kbeacon
kubectl -n "${NAMESPACE}" get pods
kubectl -n "${NAMESPACE}" get svc
