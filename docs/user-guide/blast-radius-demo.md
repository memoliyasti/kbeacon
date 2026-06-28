# Blast-radius demo

The blast-radius demo creates a small multi-namespace graph that demonstrates the core KBeacon question:

> If this Secret changes, what workloads are affected?

## What the demo creates

| Object | Namespace | Purpose |
| --- | --- | --- |
| Secret/payments-db | payments | Shared database credential. |
| Secret/stripe-api | payments | Payment provider credential. |
| Secret/platform-ca | shared | Shared certificate bundle reference. |
| Deployment/payments-api | payments | Hybrid discovery. |
| Deployment/checkout-worker | payments | Inferred discovery. |
| Deployment/reports-api | reports | Explicit discovery. |
| Secret/legacy-payment-token | payments | Intentionally missing unresolved reference. |

## Apply the demo

    ./examples/demo-blast-radius/run.sh apply

## Verify demo resources

    ./examples/demo-blast-radius/run.sh status

## Query KBeacon

Port-forward the Agent API:

    kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080

Query the blast radius for the main Secret:

    curl -sS http://127.0.0.1:8081/api/v1/secrets/payments/payments-db/impact | jq ".data"

Expected result:

- affected workloads include payments-api, checkout-worker, and reports-api;
- affected teams include payments-platform, checkout, and data-platform;
- affected namespaces include payments and reports.

Query the intentionally unresolved Secret:

    curl -sS http://127.0.0.1:8081/api/v1/secrets/payments/legacy-payment-token/impact | jq ".data.secret"

Expected result:

- exists is false;
- unresolvedReferenceCount is greater than zero.

Query workload dependencies:

    curl -sS http://127.0.0.1:8081/api/v1/workloads/payments/Deployment/payments-api/dependencies | jq ".data.dependencies"

## Prometheus checks

    kbeacon_secret_affected_workload_count{namespace="payments",secret_name="payments-db"}
    kbeacon_unresolved_secret_references{namespace="payments",secret_name="legacy-payment-token"}
    kbeacon_dependency_edges{workload_namespace="payments",workload_name="payments-api"}

## Delete the demo

    ./examples/demo-blast-radius/run.sh delete
