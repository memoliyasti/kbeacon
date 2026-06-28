#!/usr/bin/env bash
set -euo pipefail

command="${1:-apply}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
demo_dir="${root}/examples/demo-blast-radius"

apply_demo() {
  kubectl apply -f "${demo_dir}/namespace.yaml"
  kubectl apply -f "${demo_dir}/secrets.yaml"
  kubectl apply -f "${demo_dir}/workloads.yaml"

  echo
  echo "Demo resources applied."
  echo
  echo "Next:"
  echo "  kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080"
  echo "  curl -sS http://127.0.0.1:8081/api/v1/secrets/payments/payments-db/impact | jq \".data.summary\""
  echo "  curl -sS http://127.0.0.1:8081/api/v1/secrets/payments/legacy-payment-token/impact | jq \".data.secret\""
}

delete_demo() {
  kubectl delete -f "${demo_dir}/workloads.yaml" --ignore-not-found
  kubectl delete -f "${demo_dir}/secrets.yaml" --ignore-not-found
  kubectl delete -f "${demo_dir}/namespace.yaml" --ignore-not-found
}

status_demo() {
  kubectl get ns payments reports shared
  kubectl -n payments get secrets,deployments
  kubectl -n reports get deployments
  kubectl -n shared get secrets
}

case "${command}" in
  apply)
    apply_demo
    ;;
  delete)
    delete_demo
    ;;
  status)
    status_demo
    ;;
  *)
    echo "usage: $0 apply|delete|status" >&2
    exit 2
    ;;
esac
