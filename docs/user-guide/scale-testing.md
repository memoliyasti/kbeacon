# Scale testing

KBeacon includes a deterministic scale fixture generator for local performance and cardinality checks.

The generator creates:

- one namespace;
- a configurable number of Kubernetes Secrets;
- a configurable number of Deployments;
- three Secret references per Deployment: `env.secretKeyRef`, `envFrom.secretRef`, and `volumes.secret`.

## Generate a small fixture

    make scale-generate

The default output directory is:

    /tmp/kbeacon-scale-fixture

## Dry-run generated manifests

    make scale-dry-run

## Generate a custom fixture

    ./hack/generate-scale-fixture.sh /tmp/kbeacon-scale-fixture kbeacon-scale 100 500

Arguments are:

1. output directory;
2. namespace;
3. Secret count;
4. workload count.

## Apply manually

    kubectl apply -f /tmp/kbeacon-scale-fixture/namespace.yaml
    kubectl apply -f /tmp/kbeacon-scale-fixture/secrets.yaml
    kubectl apply -f /tmp/kbeacon-scale-fixture/workloads.yaml

## Observe KBeacon

After applying the fixture, observe:

    kbeacon_cluster_dependency_count
    kbeacon_cluster_secret_count
    kbeacon_cluster_workload_count
    kbeacon_graph_update_duration_seconds
    process_resident_memory_bytes

For high-cardinality Prometheus environments, test with edge metrics disabled:

    helm upgrade --install kbeacon ./charts/kbeacon \
      --namespace kbeacon-system \
      --set cluster.name=prod-eu-1 \
      --set metrics.edge.enabled=false

## Clean up

    make scale-delete

This removes the generated namespace `kbeacon-scale`.
