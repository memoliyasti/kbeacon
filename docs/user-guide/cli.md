# CLI

KBeacon ships a Kubernetes-native CLI.

The preferred executable name is `kbeacon`. The backwards-compatible executable name is `kbeaconctl`.

By default, the CLI uses your current kubeconfig context and talks to the in-cluster KBeacon Agent through the Kubernetes API server Service proxy.

No `kubectl port-forward` is required for normal CLI usage.

## Default connection

The default Agent Service target is:

~~~text
kbeacon-system/kbeacon:http
~~~

That means this works when your kubeconfig context can access the cluster and the KBeacon Agent is installed in the default namespace:

~~~bash
kbeacon ready
kbeacon get config
kbeacon get secrets --limit 20
~~~

Use a temporary namespace override when KBeacon is installed elsewhere:

~~~bash
kbeacon --namespace platform-observability ready
kbeacon -n platform-observability get dependency-map --limit 100
~~~

## Persistent defaults

Set the Agent namespace once:

~~~bash
kbeacon config set namespace kbeacon-system
~~~

Then use the CLI without repeating `--namespace`:

~~~bash
kbeacon ready
kbeacon impact report payments payments-db
~~~

Inspect and manage stored defaults:

~~~bash
kbeacon config path
kbeacon config view
kbeacon config get namespace
kbeacon config unset namespace
kbeacon config reset
~~~

Supported persistent keys:

~~~text
namespace
service
service-port
kubeconfig
context
server
~~~

## Temporary overrides

Global flags must be placed before the command:

~~~bash
kbeacon --namespace kbeacon-system ready
kbeacon --service kbeacon --service-port http ready
kbeacon --context minikube ready
kbeacon --kubeconfig ~/.kube/config ready
~~~

## Direct Agent URL mode

For local debugging or compatibility with older workflows, pass `--server`.

This disables Kubernetes service proxy mode and sends HTTP requests directly to the Agent URL.

~~~bash
kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
kbeacon --server http://127.0.0.1:8081 ready
~~~

## Common commands

Check Agent health and readiness:

~~~bash
kbeacon health
kbeacon ready
~~~

Discover API resources and graph summary:

~~~bash
kbeacon api
kbeacon get config
~~~

List graph resources:

~~~bash
kbeacon get secrets --limit 50
kbeacon get workloads --namespace payments
kbeacon get dependency-map --secret-name payments-db --limit 100
~~~

Inspect Secret blast radius:

~~~bash
kbeacon impact report payments payments-db
kbeacon impact --format json payments payments-db
~~~

Fetch workload dependencies:

~~~bash
kbeacon dependencies payments Deployment payments-api
~~~

Export and diff snapshots:

~~~bash
kbeacon snapshot export --output before.json
kbeacon snapshot export --output after.json
kbeacon snapshot diff --format markdown before.json after.json
~~~

Request a raw Agent API path:

~~~bash
kbeacon raw /api/v1/config
~~~

## RBAC notes

The CLI uses your kubeconfig credentials.

In Kubernetes proxy mode, the user or service account running the CLI must be allowed to access the KBeacon Service proxy in the target namespace.

A restricted user may need permission for the `services/proxy` subresource for the `kbeacon` Service.

The CLI does not read Kubernetes Secret values. It only queries the KBeacon Agent API, which exposes Secret names, metadata, and dependency relationships.

## Troubleshooting

Show the active CLI configuration:

~~~bash
kbeacon config view
~~~

Try an explicit namespace:

~~~bash
kbeacon --namespace kbeacon-system ready
~~~

Try a direct port-forward fallback:

~~~bash
kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
kbeacon --server http://127.0.0.1:8081 ready
~~~

If Kubernetes proxy mode returns `403`, check the current kubeconfig identity and RBAC for the KBeacon Service proxy.
