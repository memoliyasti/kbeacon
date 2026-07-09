#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-memoliyasti/kbeacon}"
HELM_REPO_NAME="${HELM_REPO_NAME:-kbeacon-release}"
HELM_REPO_URL="${HELM_REPO_URL:-https://memoliyasti.github.io/kbeacon/charts}"

RELEASE="${RELEASE:-kbeacon-upgrade-e2e}"
NAMESPACE="${NAMESPACE:-kbeacon-upgrade-e2e}"
WORKLOAD_NAMESPACE="${WORKLOAD_NAMESPACE:-kbeacon-upgrade-demo}"
CLUSTER_NAME="${CLUSTER_NAME:-upgrade-rollback-e2e}"

CURRENT_VERSION="$(awk '/^version:/ {print $2; exit}' charts/kbeacon/Chart.yaml)"
FROM_VERSION="${FROM_VERSION:-}"
TO_VERSION="${TO_VERSION:-${CURRENT_VERSION}}"
TO_CHART="${TO_CHART:-./charts/kbeacon}"
TO_IMAGE_REPOSITORY="${TO_IMAGE_REPOSITORY:-ghcr.io/memoliyasti/kbeacon}"
TO_IMAGE_TAG="${TO_IMAGE_TAG:-${TO_VERSION}}"
CLEANUP="${CLEANUP:-true}"

WORKDIR="${WORKDIR:-/tmp/kbeacon-upgrade-rollback-e2e}"
SECRET_NAME="${SECRET_NAME:-upgrade-db}"
WORKLOAD_NAME="${WORKLOAD_NAME:-upgrade-api}"

SECRET_VALUE_PLAIN="upgrade-password-should-not-leak"
SECRET_VALUE_TOKEN="upgrade-token-should-not-leak"

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

kbeacon_cli() {
  if command -v kbeacon >/dev/null 2>&1; then
    kbeacon "$@"
  else
    go run ./cmd/kbeaconctl "$@"
  fi
}

cleanup() {
  if [ "${CLEANUP}" = "true" ]; then
    helm uninstall "${RELEASE}" -n "${NAMESPACE}" >/dev/null 2>&1 || true
    kubectl delete namespace "${NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
    kubectl delete namespace "${WORKLOAD_NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT

require helm
require kubectl
require jq
require grep
require awk
require sed

rm -rf "${WORKDIR}"
mkdir -p "${WORKDIR}"

echo "===== context ====="
kubectl config current-context
kubectl get nodes -o wide

echo
echo "===== resolve source version ====="
helm repo add "${HELM_REPO_NAME}" "${HELM_REPO_URL}" >/dev/null 2>&1 || true
helm repo update "${HELM_REPO_NAME}"

if [ -z "${FROM_VERSION}" ]; then
  FROM_VERSION="$(
    helm search repo "${HELM_REPO_NAME}/kbeacon" --versions |
      awk -v current="${CURRENT_VERSION}" 'NR > 1 && $2 != current {print $2; exit}'
  )"
fi

if [ -z "${FROM_VERSION}" ]; then
  echo "could not resolve FROM_VERSION from public Helm repo" >&2
  exit 1
fi

echo "FROM_VERSION=${FROM_VERSION}"
echo "TO_VERSION=${TO_VERSION}"
echo "TO_CHART=${TO_CHART}"
echo "TO_IMAGE_REPOSITORY=${TO_IMAGE_REPOSITORY}"
echo "TO_IMAGE_TAG=${TO_IMAGE_TAG}"

echo
echo "===== install previous public release ====="
helm upgrade --install "${RELEASE}" "${HELM_REPO_NAME}/kbeacon" \
  --version "${FROM_VERSION}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --set cluster.name="${CLUSTER_NAME}-before" \
  --set dashboards.enabled=true \
  --set serviceMonitor.enabled=false \
  --set prometheus.scrapeAnnotations.enabled=true \
  --wait \
  --timeout 10m

kubectl -n "${NAMESPACE}" rollout status "deploy/${RELEASE}" --timeout=5m

kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" ready | jq -e '.status == "ready"'
kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" get config | jq .

echo
echo "===== create workload fixture ====="
kubectl create namespace "${WORKLOAD_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${WORKLOAD_NAMESPACE}" create secret generic "${SECRET_NAME}" \
  --from-literal=password="${SECRET_VALUE_PLAIN}" \
  --from-literal=token="${SECRET_VALUE_TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f -

cat > "${WORKDIR}/workload.yaml" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${WORKLOAD_NAME}
  namespace: ${WORKLOAD_NAMESPACE}
  labels:
    app.kubernetes.io/name: ${WORKLOAD_NAME}
    app.kubernetes.io/team: upgrade-platform
    app.kubernetes.io/criticality: high
  annotations:
    kbeacon.io/discovery-mode: hybrid
    kbeacon.io/watch-secrets: ${SECRET_NAME}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${WORKLOAD_NAME}
  template:
    metadata:
      labels:
        app: ${WORKLOAD_NAME}
    spec:
      containers:
        - name: app
          image: busybox:1.36
          command:
            - sh
            - -c
            - sleep 3600
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: ${SECRET_NAME}
                  key: password
          envFrom:
            - secretRef:
                name: ${SECRET_NAME}
          volumeMounts:
            - name: db-secret
              mountPath: /var/run/secrets/db
              readOnly: true
      volumes:
        - name: db-secret
          secret:
            secretName: ${SECRET_NAME}
EOF

kubectl apply -f "${WORKDIR}/workload.yaml"
kubectl -n "${WORKLOAD_NAMESPACE}" rollout status "deploy/${WORKLOAD_NAME}" --timeout=5m

echo
echo "===== verify previous release graph ====="
sleep 45

kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" get secrets --limit 300 > "${WORKDIR}/before-secrets.json"
kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" get workloads --limit 300 > "${WORKDIR}/before-workloads.json"
kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" get dependency-map --limit 1000 > "${WORKDIR}/before-dependency-map.json"
kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" impact --format json "${WORKLOAD_NAMESPACE}" "${SECRET_NAME}" > "${WORKDIR}/before-impact.json"
kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" snapshot export --output "${WORKDIR}/before-snapshot.json"

jq -e '.data.summary.affectedWorkloadCount >= 1' "${WORKDIR}/before-impact.json"
test -s "${WORKDIR}/before-snapshot.json"

if grep -RniE "${SECRET_VALUE_PLAIN}|${SECRET_VALUE_TOKEN}|$(printf '%s' "${SECRET_VALUE_PLAIN}" | base64)|$(printf '%s' "${SECRET_VALUE_TOKEN}" | base64)" "${WORKDIR}"; then
  echo "FAIL: secret value leaked before upgrade" >&2
  exit 1
fi

echo
echo "===== upgrade to local chart/current release ====="
helm upgrade --install "${RELEASE}" "${TO_CHART}" \
  --namespace "${NAMESPACE}" \
  --set cluster.name="${CLUSTER_NAME}-after" \
  --set image.repository="${TO_IMAGE_REPOSITORY}" \
  --set image.tag="${TO_IMAGE_TAG}" \
  --set image.pullPolicy=IfNotPresent \
  --set dashboards.enabled=true \
  --set serviceMonitor.enabled=false \
  --set prometheus.scrapeAnnotations.enabled=true \
  --wait \
  --timeout 10m

kubectl -n "${NAMESPACE}" rollout status "deploy/${RELEASE}" --timeout=5m

kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" ready | jq -e '.status == "ready"'
kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" get config | jq .

echo
echo "===== verify upgraded graph ====="
sleep 45

kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" get secrets --limit 300 > "${WORKDIR}/after-secrets.json"
kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" get workloads --limit 300 > "${WORKDIR}/after-workloads.json"
kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" get dependency-map --limit 1000 > "${WORKDIR}/after-dependency-map.json"
kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" impact --format json "${WORKLOAD_NAMESPACE}" "${SECRET_NAME}" > "${WORKDIR}/after-impact.json"
kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" snapshot export --output "${WORKDIR}/after-snapshot.json"

jq -e '.data.summary.affectedWorkloadCount >= 1' "${WORKDIR}/after-impact.json"
test -s "${WORKDIR}/after-snapshot.json"

kbeacon_cli snapshot diff --format markdown "${WORKDIR}/before-snapshot.json" "${WORKDIR}/after-snapshot.json" > "${WORKDIR}/snapshot-diff.md"
test -s "${WORKDIR}/snapshot-diff.md"

if grep -RniE "${SECRET_VALUE_PLAIN}|${SECRET_VALUE_TOKEN}|$(printf '%s' "${SECRET_VALUE_PLAIN}" | base64)|$(printf '%s' "${SECRET_VALUE_TOKEN}" | base64)" "${WORKDIR}"; then
  echo "FAIL: secret value leaked after upgrade" >&2
  exit 1
fi

echo
echo "===== rollback ====="
PREVIOUS_REVISION="$(
  helm history "${RELEASE}" -n "${NAMESPACE}" -o json |
    jq -r 'sort_by(.revision) | .[-2].revision'
)"

if [ -z "${PREVIOUS_REVISION}" ] || [ "${PREVIOUS_REVISION}" = "null" ]; then
  echo "could not resolve previous Helm revision" >&2
  helm history "${RELEASE}" -n "${NAMESPACE}" || true
  exit 1
fi

echo "PREVIOUS_REVISION=${PREVIOUS_REVISION}"

helm rollback "${RELEASE}" "${PREVIOUS_REVISION}" \
  -n "${NAMESPACE}" \
  --wait \
  --timeout 10m

kubectl -n "${NAMESPACE}" rollout status "deploy/${RELEASE}" --timeout=5m

kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" ready | jq -e '.status == "ready"'
kbeacon_cli --namespace "${NAMESPACE}" --service "${RELEASE}" impact --format json "${WORKLOAD_NAMESPACE}" "${SECRET_NAME}" > "${WORKDIR}/rollback-impact.json"
jq -e '.data.summary.affectedWorkloadCount >= 1' "${WORKDIR}/rollback-impact.json"

if grep -RniE "${SECRET_VALUE_PLAIN}|${SECRET_VALUE_TOKEN}|$(printf '%s' "${SECRET_VALUE_PLAIN}" | base64)|$(printf '%s' "${SECRET_VALUE_TOKEN}" | base64)" "${WORKDIR}"; then
  echo "FAIL: secret value leaked after rollback" >&2
  exit 1
fi

echo
echo "===== helm history ====="
helm history "${RELEASE}" -n "${NAMESPACE}"

echo
echo "OK: upgrade, snapshot diff, rollback, CLI readiness, impact, and no-secret-leak e2e passed"
