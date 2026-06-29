#!/usr/bin/env bash
set -euo pipefail

secret_pattern="READMEkbeacon|kbeacon-2|ghcr\.io/.*/kbeacon-agent|github\.com/kbeacon/kbeacon|GHCR_TOKEN=|ghp_|github_pat_|BEGIN .*PRIVATE KEY"
secret_hits="$(git grep -nE "${secret_pattern}" -- . ":!Makefile" ":!hack/stale-check.sh" ":!CHANGELOG.md" || true)"

if [ -n "${secret_hits}" ]; then
  echo "stale-check failed: stale references or secret-like tokens found"
  echo
  echo "${secret_hits}"
  exit 1
fi

version_pattern="0\.2\.0|v0\.2\.0|0\.2\.1|v0\.2\.1"
version_hits="$(git grep -nE "${version_pattern}" -- README.md RELEASE.md charts docs mkdocs.yml PROJECT_STRUCTURE.md .github internal cmd examples 2>/dev/null || true)"

if [ -n "${version_hits}" ]; then
  echo "stale-check failed: old v0.2.0 references found outside CHANGELOG.md"
  echo
  echo "${version_hits}"
  exit 1
fi

echo "stale-check passed"
