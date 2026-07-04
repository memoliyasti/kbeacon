# KBeacon CLI

`kbeaconctl` is a small command-line client for the read-only KBeacon Agent API.

It is intended for platform and SRE workflows where a shell-friendly client is easier than hand-written `curl` commands.

## Server address

By default, `kbeaconctl` connects to:

```text
http://127.0.0.1:8081
```

Use a port-forward during local access:

```bash
kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
```

Then run:

```bash
kbeaconctl ready
kbeaconctl get secrets --limit 20
kbeaconctl get workloads --namespace payments
kbeaconctl get dependency-map --secret-name payments-db --resolved true
kbeaconctl impact payments payments-db
kbeaconctl impact report payments payments-db
kbeaconctl snapshot export --output kbeacon-snapshot.json
```

You can also set the server explicitly:

```bash
kbeaconctl --server http://127.0.0.1:8081 ready
```

Or by environment variable:

```bash
export KBEACONCTL_SERVER=http://127.0.0.1:8081
kbeaconctl get secrets
```

## Commands

| Command | Purpose |
| --- | --- |
| `version` | Print CLI version metadata. |
| `health` | Query `/healthz`. |
| `ready` | Query `/readyz`. |
| `api` | Query API discovery at `/api/v1`. |
| `get secrets` | List observed and referenced Secrets. |
| `get workloads` | List normalized workloads. |
| `get dependency-map` | Query the current dependency graph. |
| `get config` | Query Agent graph summary. |
| `impact <namespace> <secret>` | Query Secret impact details as JSON. |
| `impact report <namespace> <secret>` | Print a human-readable Secret impact report. |
| `dependencies <namespace> <kind> <name>` | Query workload dependencies. |
| `snapshot export` | Export a portable JSON snapshot from the Agent API. |
| `snapshot diff` | Compare two exported snapshots. |
| `raw <path>` | Query an arbitrary Agent API path. |

## Filtering

The list and dependency-map commands pass supported Agent API filters through to the server.

Useful filters include:

```bash
kbeaconctl get secrets --namespace payments --exists true
kbeaconctl get workloads --workload-kind Deployment --discovery-mode hybrid
kbeaconctl get dependency-map --secret-name payments-db --resolved true
kbeaconctl get dependency-map --owner-team payments-platform --criticality critical
```

Pagination is also supported:

```bash
kbeaconctl get secrets --limit 100 --offset 200
```

## Secret impact report

Use the report form when reviewing a Secret rotation or preparing a change review:

```bash
kbeaconctl impact report payments payments-db
```

The same report can be selected with a format flag:

```bash
kbeaconctl impact --format report payments payments-db
```

The report includes:

- Secret identity and impact score;
- affected workload, team, namespace, and unresolved-reference counts;
- discovery mode distribution;
- affected teams;
- affected workloads;
- dependency edges and source types.

The plain `impact <namespace> <secret>` form still prints the original Agent API JSON response for scripts and automation.

## Release binaries

Semantic KBeacon releases publish kbeaconctl binaries for Linux and macOS alongside the Agent binaries. Binary names follow this pattern:

    kbeaconctl_vX.Y.Z_linux_amd64
    kbeaconctl_vX.Y.Z_linux_arm64
    kbeaconctl_vX.Y.Z_darwin_amd64
    kbeaconctl_vX.Y.Z_darwin_arm64

Use the release checksums.txt file to verify downloaded CLI binaries.

## Snapshot export

Use snapshot export when you need a portable point-in-time view of the Agent API for offline review, support bundles, CI artifacts, or later diffing.

    kbeaconctl snapshot export --output kbeacon-snapshot.json

By default, the snapshot includes:

- Agent graph summary from `/api/v1/config`;
- Secrets from `/api/v1/secrets`;
- workloads from `/api/v1/workloads`;
- dependency map from `/api/v1/dependency-map`.

You can export only selected resources:

    kbeaconctl snapshot export --include secrets,dependency-map --output dependencies.json

Use `--output -` to write JSON to stdout.

## Snapshot diff

Compare two exported KBeacon snapshots:

    kbeaconctl snapshot diff old-snapshot.json new-snapshot.json

Emit machine-readable JSON:

    kbeaconctl snapshot diff --format json old-snapshot.json new-snapshot.json

    kbeaconctl snapshot diff --format markdown old-snapshot.json new-snapshot.json

Limit the comparison to selected resources:

    kbeaconctl snapshot diff --include secrets,edges old-snapshot.json new-snapshot.json

Fail CI when any change is detected:

    kbeaconctl snapshot diff --fail-on-change old-snapshot.json new-snapshot.json

The diff reports added, removed, and changed Secrets, workloads, and dependency edges. Snapshot diff is offline and does not contact the Agent API.

Markdown output is intended for pull request comments and CI summaries:

    kbeaconctl snapshot diff --format markdown old-snapshot.json new-snapshot.json > snapshot-diff.md
