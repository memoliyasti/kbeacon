#!/usr/bin/env bash
set -euo pipefail

rules="examples/prometheus/rules.yaml"
page="docs/operator-guide/runbooks.md"

test -s "${rules}"
test -s "${page}"

alerts="$(awk '/^[[:space:]]*- alert: / {print $3}' "${rules}")"
count=0

for alert in ${alerts}
do
  count=$((count + 1))
  grep -Fq "## ${alert}" "${page}"
  anchor="$(printf "%s" "${alert}" | tr '[:upper:]' '[:lower:]')"
  grep -Fq "runbook_url: \"https://memoliyasti.github.io/kbeacon/operator-guide/runbooks/#${anchor}\"" "${rules}"
done

if [ "${count}" -eq 0 ]; then
  echo "ERROR: no Prometheus alerts found in ${rules}"
  exit 1
fi

runbook_count="$(grep -c "runbook_url:" "${rules}")"

if [ "${runbook_count}" -ne "${count}" ]; then
  echo "ERROR: expected ${count} runbook_url annotations, found ${runbook_count}"
  exit 1
fi

grep -Fq "Runbooks: operator-guide/runbooks.md" mkdocs.yml
grep -Fq "docs/operator-guide/runbooks.md" README.md
grep -Fq "operator-guide/runbooks.md" docs/user-guide/alerting.md

echo "runbook validation passed: alerts=${count}"
