#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-kube-addons}"

helm upgrade prometheus prometheus-community/prometheus \
  --namespace "${NAMESPACE}" \
  --reset-values \
  --values hack/local-dev/prometheus-kbeacon-incluster-values.yaml

kubectl -n "${NAMESPACE}" rollout status deploy/prometheus-server
kubectl -n "${NAMESPACE}" get pods
