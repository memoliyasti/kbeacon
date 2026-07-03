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
| `impact <namespace> <secret>` | Query Secret impact details. |
| `dependencies <namespace> <kind> <name>` | Query workload dependencies. |
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

## Output

The first version of `kbeaconctl` prints the Agent API JSON response as-is.

Human-readable reports are tracked as the next CLI step.
