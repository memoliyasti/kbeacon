#!/usr/bin/env bash
set -euo pipefail

CHART_VERSION="$(awk "/^version:/ {print \$2; exit}" charts/kbeacon/Chart.yaml)"

echo "CHART_VERSION=${CHART_VERSION}"

grep -q "version: ${CHART_VERSION}" charts/kbeacon/Chart.yaml
grep -q "appVersion:.*${CHART_VERSION}" charts/kbeacon/Chart.yaml
grep -q "tag:.*${CHART_VERSION}" charts/kbeacon/values.yaml
grep -q "version: ${CHART_VERSION}" docs/api/openapi.yaml

for file in README.md RELEASE.md docs/getting-started.md docs/user-guide/installation.md docs/operator-guide/releases.md docs/reference/helm.md; do
  if [ -f "${file}" ]; then
    if grep -nE "0\.3\.1[0-7]|v0\.3\.1[0-7]|0\.2\.|v0\.2\.|0\.1\.|v0\.1\." "${file}"; then
      echo "FAIL: stale version reference in ${file}"
      exit 1
    fi
  fi
done

if grep -nE "tag: \"0\.[0-9]+\.[0-9]+\"" charts/kbeacon/README.md; then
  echo "FAIL: charts/kbeacon/README.md has hard-coded image tag"
  exit 1
fi

echo "OK: local version and public-doc consistency lint passed"
