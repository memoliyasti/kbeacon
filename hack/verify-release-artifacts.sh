#!/usr/bin/env bash
set -euo pipefail

TAG="${1:-}"
REPO="${REPO:-memoliyasti/kbeacon}"
HELM_REPO_NAME="${HELM_REPO_NAME:-kbeacon-release}"
HELM_REPO_URL="${HELM_REPO_URL:-https://memoliyasti.github.io/kbeacon/charts}"
IMAGE="${IMAGE:-ghcr.io/memoliyasti/kbeacon}"

if [ -z "${TAG}" ]; then
  TAG="$(gh release view --repo "${REPO}" --json tagName --jq .tagName)"
fi

VERSION="${TAG#v}"

echo "TAG=${TAG}"
echo "VERSION=${VERSION}"

test -n "${TAG}"
test -n "${VERSION}"

echo
echo "===== release metadata ====="
gh release view "${TAG}" --repo "${REPO}" --json tagName,name,isDraft,isPrerelease,publishedAt,url --jq .

echo
echo "===== release assets ====="
gh release view "${TAG}" --repo "${REPO}" --json assets --jq ".assets[].name" | sort | tee /tmp/kbeacon-release-assets.txt

for asset in \
  checksums.txt \
  "kbeacon-${VERSION}.tgz" \
  "kbeacon-${VERSION}.tgz.prov" \
  "kbeacon-${VERSION}.tgz.spdx.json" \
  "kbeacon-source-${TAG}.spdx.json" \
  "kbeacon_${TAG}_darwin_amd64" \
  "kbeacon_${TAG}_darwin_arm64" \
  "kbeacon_${TAG}_linux_amd64" \
  "kbeacon_${TAG}_linux_arm64" \
  "kbeaconctl_${TAG}_darwin_amd64" \
  "kbeaconctl_${TAG}_darwin_arm64" \
  "kbeaconctl_${TAG}_linux_amd64" \
  "kbeaconctl_${TAG}_linux_arm64" \
  "kbeacon-agent_${TAG}_darwin_amd64" \
  "kbeacon-agent_${TAG}_darwin_arm64" \
  "kbeacon-agent_${TAG}_linux_amd64" \
  "kbeacon-agent_${TAG}_linux_arm64"
do
  echo "checking asset ${asset}"
  grep -Fx "${asset}" /tmp/kbeacon-release-assets.txt >/dev/null
done

echo
echo "===== GHCR tags ====="
docker pull "${IMAGE}:${VERSION}"
docker pull "${IMAGE}:${TAG}"
DIGEST_VERSION="$(docker image inspect "${IMAGE}:${VERSION}" --format "{{index .RepoDigests 0}}")"
DIGEST_TAG="$(docker image inspect "${IMAGE}:${TAG}" --format "{{index .RepoDigests 0}}")"
echo "${VERSION}=${DIGEST_VERSION}"
echo "${TAG}=${DIGEST_TAG}"
test "${DIGEST_VERSION}" = "${DIGEST_TAG}"

echo
echo "===== public Helm index ====="
curl -fsSL -H "Cache-Control: no-cache" "${HELM_REPO_URL}/index.yaml?ts=$(date +%s)" -o /tmp/kbeacon-remote-chart-index.yaml
grep -n "version: ${VERSION}" /tmp/kbeacon-remote-chart-index.yaml
grep -n "kbeacon-${VERSION}.tgz" /tmp/kbeacon-remote-chart-index.yaml

helm repo remove "${HELM_REPO_NAME}" 2>/dev/null || true
HELM_CACHE="$(helm env HELM_REPOSITORY_CACHE)"
rm -f "${HELM_CACHE}/${HELM_REPO_NAME}-index.yaml"
rm -f "${HELM_CACHE}/${HELM_REPO_NAME}-charts.txt"
helm repo add "${HELM_REPO_NAME}" "${HELM_REPO_URL}"
helm repo update "${HELM_REPO_NAME}"
helm search repo "${HELM_REPO_NAME}/kbeacon" --versions | head -10
helm show chart "${HELM_REPO_NAME}/kbeacon" --version "${VERSION}"

echo
echo "===== chart README and values from public chart ====="
helm show readme "${HELM_REPO_NAME}/kbeacon" --version "${VERSION}" | grep -nA10 -B3 "## Image" || true
if helm show readme "${HELM_REPO_NAME}/kbeacon" --version "${VERSION}" | grep -nE "tag: \"0\.[0-9]+\.[0-9]+\""; then
  echo "FAIL: chart README contains hard-coded image tag"
  exit 1
fi
helm show values "${HELM_REPO_NAME}/kbeacon" --version "${VERSION}" | grep -nE "repository:|tag:|pullPolicy:" | head -20
helm show values "${HELM_REPO_NAME}/kbeacon" --version "${VERSION}" | grep -q "tag: \"${VERSION}\""

echo
echo "===== local source metadata if running from repo ====="
if [ -f charts/kbeacon/Chart.yaml ]; then
  grep -q "version: ${VERSION}" charts/kbeacon/Chart.yaml
  grep -q "appVersion:.*${VERSION}" charts/kbeacon/Chart.yaml
  grep -q "tag:.*${VERSION}" charts/kbeacon/values.yaml
  grep -q "version: ${VERSION}" docs/api/openapi.yaml
fi

echo
echo "OK: release artifacts, GHCR tags, Helm index, chart README, and source metadata are consistent"
