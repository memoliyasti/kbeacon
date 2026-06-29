#!/usr/bin/env bash
set -euo pipefail

files="$(find dashboards charts/kbeacon/dashboards -type f -name "*.json" 2>/dev/null | sort || true)"

if [ -z "${files}" ]; then
  echo "dashboard validation failed: no dashboard JSON files found"
  exit 1
fi

for file in ${files}; do
  python3 -c 'import json, sys; p=sys.argv[1]; data=json.load(open(p, encoding="utf-8")); assert isinstance(data, dict), f"{p}: dashboard root must be a JSON object"; assert data.get("title") or data.get("uid"), f"{p}: dashboard should include title or uid"; print("ok dashboard", p)' "${file}"
done

metric_count="$(grep -Rho "kbeacon_[a-zA-Z0-9_:]*" dashboards charts/kbeacon/dashboards 2>/dev/null | sort -u | wc -l | tr -d " ")"

if [ "${metric_count}" -lt 3 ]; then
  echo "dashboard validation failed: expected at least 3 unique KBeacon metric references, got ${metric_count}"
  exit 1
fi

echo "dashboard validation passed: files=$(printf "%s\n" ${files} | wc -l | tr -d " ") unique_metrics=${metric_count}"
