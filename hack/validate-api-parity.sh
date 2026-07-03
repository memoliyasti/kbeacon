#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
import json
import re
from pathlib import Path

errors = []

def fail(message: str) -> None:
    errors.append(message)

def require(condition: bool, message: str) -> None:
    if not condition:
        fail(message)

def read(path: str) -> str:
    p = Path(path)
    require(p.exists(), f"missing required file: {path}")
    return p.read_text() if p.exists() else ""

server = read("internal/server/server.go")
openapi = read("docs/api/openapi.yaml")
api_docs = read("docs/api/index.md")
developer_api_docs = read("docs/developer-guide/api.md")

expected = {
    "listSecrets": {
        "endpoint": "/api/v1/secrets",
        "go_func": "listSecrets",
        "params": [
            "namespace",
            "ownerTeam",
            "criticality",
            "secretName",
            "exists",
            "limit",
            "offset",
        ],
        "refs": [
            "Namespace",
            "OwnerTeam",
            "Criticality",
            "SecretName",
            "Exists",
            "Limit",
            "Offset",
        ],
    },
    "listWorkloads": {
        "endpoint": "/api/v1/workloads",
        "go_func": "listWorkloads",
        "params": [
            "namespace",
            "ownerTeam",
            "criticality",
            "workloadKind",
            "workloadName",
            "discoveryMode",
            "limit",
            "offset",
        ],
        "refs": [
            "Namespace",
            "OwnerTeam",
            "Criticality",
            "WorkloadKind",
            "WorkloadName",
            "DiscoveryMode",
            "Limit",
            "Offset",
        ],
    },
    "getDependencyMap": {
        "endpoint": "/api/v1/dependency-map",
        "go_func": "dependencyMap",
        "params": [
            "namespace",
            "workloadNamespace",
            "secretNamespace",
            "workloadKind",
            "workloadName",
            "secretName",
            "ownerTeam",
            "criticality",
            "resolved",
            "discoveryMode",
            "limit",
            "offset",
        ],
        "refs": [
            "Namespace",
            "WorkloadNamespace",
            "SecretNamespace",
            "WorkloadKind",
            "WorkloadName",
            "SecretName",
            "OwnerTeam",
            "Criticality",
            "Resolved",
            "DiscoveryMode",
            "Limit",
            "Offset",
        ],
    },
}

def go_func_body(name: str) -> str:
    marker = f"func (s *Server) {name}"
    start = server.find(marker)
    if start < 0:
        fail(f"server function not found: {name}")
        return ""
    next_func = server.find("\nfunc ", start + 1)
    if next_func < 0:
        next_func = len(server)
    return server[start:next_func]

def openapi_operation_section(operation_id: str) -> str:
    marker = f"operationId: {operation_id}"
    start = openapi.find(marker)
    if start < 0:
        fail(f"OpenAPI operationId not found: {operation_id}")
        return ""
    candidates = [
        pos for pos in [
            openapi.find("\n  /", start + 1),
            openapi.find("\ncomponents:", start + 1),
        ] if pos >= 0
    ]
    end = min(candidates) if candidates else len(openapi)
    return openapi[start:end]

server_param_markers = {
    "namespace": 'q.Get("namespace")',
    "ownerTeam": 'q.Get("ownerTeam")',
    "criticality": 'q.Get("criticality")',
    "secretName": 'q.Get("secretName")',
    "secretNamespace": 'q.Get("secretNamespace")',
    "workloadNamespace": 'q.Get("workloadNamespace")',
    "workloadKind": 'q.Get("workloadKind")',
    "workloadName": 'q.Get("workloadName")',
    "discoveryMode": 'q.Get("discoveryMode")',
    "exists": 'boolQueryParam(q, "exists")',
    "resolved": 'boolQueryParam(q, "resolved")',
    "limit": 'values.Get("limit")',
    "offset": 'values.Get("offset")',
}

for operation_id, spec in expected.items():
    body = go_func_body(spec["go_func"])
    section = openapi_operation_section(operation_id)

    require("parsePagination(q)" in body, f"{spec['go_func']} must parse pagination")
    require("writeEnvelopeWithPagination" in body, f"{spec['go_func']} must return pagination envelope")

    for param in spec["params"]:
        marker = server_param_markers[param]
        require(marker in server, f"server does not appear to handle query parameter {param!r}")

    for ref in spec["refs"]:
        component_marker = f"\n    {ref}:\n"
        require(component_marker in openapi, f"OpenAPI component parameter missing: {ref}")
        operation_marker = f"#/components/parameters/{ref}"
        require(operation_marker in section, f"OpenAPI operation {operation_id} missing parameter ref {ref}")

    for doc in [api_docs, developer_api_docs]:
        require(spec["endpoint"] in doc, f"documentation missing endpoint {spec['endpoint']}")
        for param in spec["params"]:
            require(param in doc, f"documentation for {spec['endpoint']} missing parameter {param}")

require("pagination:" in openapi, "OpenAPI envelope must include pagination")
require("Pagination:" in openapi, "OpenAPI Pagination schema missing")
for field in ["limit", "offset", "total", "returned", "nextOffset"]:
    require(field in openapi, f"OpenAPI Pagination schema missing field {field}")

example_expectations = {
    "examples/api/secrets-list.json": True,
    "examples/api/workloads-list.json": True,
    "examples/api/dependency-map.json": True,
    "examples/api/secret-impact.json": False,
    "examples/api/workload-dependencies.json": False,
}

for path, should_have_pagination in example_expectations.items():
    p = Path(path)
    require(p.exists(), f"missing API example: {path}")
    if not p.exists():
        continue

    try:
        data = json.loads(p.read_text())
    except Exception as exc:
        fail(f"invalid JSON example {path}: {exc}")
        continue

    has_pagination = "pagination" in data
    if should_have_pagination:
        require(has_pagination, f"{path} should include top-level pagination")
        if has_pagination:
            pagination = data["pagination"]
            for field in ["limit", "offset", "total", "returned"]:
                require(field in pagination, f"{path} pagination missing field {field}")
    else:
        require(not has_pagination, f"{path} should not include pagination")

if errors:
    print("API parity validation failed:")
    for error in errors:
        print(f"- {error}")
    raise SystemExit(1)

print("api parity validation passed")
PY
