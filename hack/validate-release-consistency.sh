#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

require_file() {
  local path="$1"
  [ -s "${path}" ] || fail "missing required file: ${path}"
}

require_file README.md
require_file CHANGELOG.md
require_file RELEASE.md
require_file charts/kbeacon/Chart.yaml
require_file charts/kbeacon/values.yaml
require_file charts/kbeacon/README.md
require_file docs/api/openapi.yaml
require_file mkdocs.yml

CHART_VERSION="$(awk '/^version:/ {print $2; exit}' charts/kbeacon/Chart.yaml)"
APP_VERSION="$(awk '/^appVersion:/ {gsub(/"/, "", $2); print $2; exit}' charts/kbeacon/Chart.yaml)"
VALUES_TAG="$(awk '/^[[:space:]]+tag:/ {gsub(/"/, "", $2); print $2; exit}' charts/kbeacon/values.yaml)"
OPENAPI_VERSION="$(awk '/^[[:space:]]+version:/ {print $2; exit}' docs/api/openapi.yaml)"
RELEASE_TAG="v${CHART_VERSION}"

printf 'CHART_VERSION=%s\n' "${CHART_VERSION}"
printf 'APP_VERSION=%s\n' "${APP_VERSION}"
printf 'VALUES_TAG=%s\n' "${VALUES_TAG}"
printf 'OPENAPI_VERSION=%s\n' "${OPENAPI_VERSION}"
printf 'RELEASE_TAG=%s\n' "${RELEASE_TAG}"

[ -n "${CHART_VERSION}" ] || fail "Chart.yaml version is empty"
[ "${APP_VERSION}" = "${CHART_VERSION}" ] || fail "Chart appVersion ${APP_VERSION} does not match chart version ${CHART_VERSION}"
[ "${VALUES_TAG}" = "${CHART_VERSION}" ] || fail "values.yaml image.tag ${VALUES_TAG} does not match chart version ${CHART_VERSION}"
[ "${OPENAPI_VERSION}" = "${CHART_VERSION}" ] || fail "OpenAPI version ${OPENAPI_VERSION} does not match chart version ${CHART_VERSION}"

grep -Fq "## ${RELEASE_TAG}" CHANGELOG.md || fail "CHANGELOG.md is missing ${RELEASE_TAG}"

README_RELEASE="$(
  awk '
    found && $0 ~ /^v[0-9]+\.[0-9]+\.[0-9]+$/ { print; exit }
    /Current release line:/ { found=1 }
  ' README.md
)"

[ "${README_RELEASE}" = "${RELEASE_TAG}" ] || fail "README current release line is ${README_RELEASE:-missing}, expected ${RELEASE_TAG}"

grep -Fq 'tag: "<release version>"' charts/kbeacon/README.md || fail "charts/kbeacon/README.md should use tag: \"<release version>\""

if grep -nE 'tag: "0\.3\.[0-9]+"' charts/kbeacon/README.md; then
  fail "charts/kbeacon/README.md contains a hard-coded 0.3.x image tag"
fi

if [ -f docs/operator-guide/access-control.md ]; then
  grep -Fq "Access control: operator-guide/access-control.md" mkdocs.yml || fail "mkdocs.yml does not include access-control doc"
fi

if [ -f docs/operator-guide/upgrades.md ]; then
  grep -Fq "Upgrades and rollback: operator-guide/upgrades.md" mkdocs.yml || fail "mkdocs.yml does not include upgrades doc"
fi

if [ -f docs/reference/benchmarks.md ]; then
  grep -Fq "Benchmarks: reference/benchmarks.md" mkdocs.yml || fail "mkdocs.yml does not include benchmarks doc"
fi

for path in hack/verify-release-artifacts.sh hack/post-release-smoke.sh hack/validate-release-consistency.sh; do
  require_file "${path}"
  [ -x "${path}" ] || fail "${path} is not executable"
  bash -n "${path}"
done

require_file .github/workflows/release-smoke.yaml
grep -Fq "hack/verify-release-artifacts.sh" .github/workflows/release-smoke.yaml || fail "release-smoke workflow does not verify public artifacts"
grep -Fq "hack/post-release-smoke.sh" .github/workflows/release-smoke.yaml || fail "release-smoke workflow does not run post-release smoke"

python3 - "${CHART_VERSION}" <<'KBEACON_RELEASE_TOKEN_CHECK_PY'
from pathlib import Path
import re
import sys

current = sys.argv[1]
allowed = {current, "v" + current}

paths = []
seen = set()

def add(path: str) -> None:
    p = Path(path)
    if p.exists() and p.is_file() and path not in seen:
        seen.add(path)
        paths.append(p)

for path in [
    "README.md",
    "RELEASE.md",
    "charts/kbeacon/README.md",
    "charts/kbeacon/values.yaml",
    "docs/api/openapi.yaml",
    "docs/getting-started.md",
    "docs/user-guide/installation.md",
    "docs/operator-guide/releases.md",
    "docs/reference/helm.md",
]:
    add(path)

docs = Path("docs")
if docs.exists():
    for p in sorted(docs.rglob("*.md")):
        add(p.as_posix())

stale = []
pattern = re.compile(r"\bv?0\.3\.\d+\b")

for path in paths:
    text = path.read_text(encoding="utf-8")
    for match in pattern.finditer(text):
        token = match.group(0)
        if token not in allowed:
            line = text.count("\n", 0, match.start()) + 1
            stale.append(f"{path}:{line}: stale release token {token}; expected {current} or v{current}")

if stale:
    print("\n".join(stale))
    sys.exit(1)
KBEACON_RELEASE_TOKEN_CHECK_PY

install_docs=(
  README.md
  docs/getting-started.md
  docs/user-guide/installation.md
  charts/kbeacon/README.md
  examples/demo-blast-radius/README.md
)

existing_install_docs=()
for path in "${install_docs[@]}"; do
  if [ -f "${path}" ]; then
    existing_install_docs+=("${path}")
  fi
done

if [ "${#existing_install_docs[@]}" -gt 0 ]; then
  if grep -nE 'port-forward svc/kbeacon|127\.0\.0\.1:8081|curl -sS http://127\.0\.0\.1:8081' "${existing_install_docs[@]}"; then
    fail "install/demo docs should stay kube-native CLI first, not port-forward first"
  fi
fi

printf 'OK: release metadata, public docs, chart README, install docs, and release smoke references are consistent\n'
