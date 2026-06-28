# KBeacon blast-radius demo

This demo creates a small multi-namespace application graph that shows why KBeacon exists.

It creates:

- Secret/payments-db in namespace payments.
- Secret/stripe-api in namespace payments.
- Secret/platform-ca in namespace shared.
- Deployment/payments-api.
- Deployment/checkout-worker.
- Deployment/reports-api.
- One intentionally unresolved dependency: Secret/legacy-payment-token.

Expected blast-radius story:

| Secret | Expected impact |
| --- | --- |
| payments/payments-db | Referenced by payments-api, checkout-worker, and reports-api. |
| payments/stripe-api | Referenced by checkout-worker. |
| shared/platform-ca | Referenced explicitly by payments-api and reports-api. |
| payments/legacy-payment-token | Referenced explicitly but intentionally missing, so it should appear as unresolved. |

## Apply demo resources

    ./examples/demo-blast-radius/run.sh apply

## Inspect resources

    ./examples/demo-blast-radius/run.sh status

## Query KBeacon

Install or upgrade KBeacon first, then port-forward the Agent API.

    kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080

In another terminal:

    curl -sS http://127.0.0.1:8081/api/v1/secrets/payments/payments-db/impact | jq ".data.summary"
    curl -sS http://127.0.0.1:8081/api/v1/secrets/payments/legacy-payment-token/impact | jq ".data.secret"
    curl -sS http://127.0.0.1:8081/api/v1/workloads/payments/Deployment/payments-api/dependencies | jq ".data.dependencies"

## Delete demo resources

    ./examples/demo-blast-radius/run.sh delete
