# Alert runbooks

This page documents the operational runbooks linked from KBeacon Prometheus alert rules.

KBeacon alerts are Secret dependency intelligence signals. They do not expose Secret values, but Secret names, workload names, namespaces, and owner metadata can still be sensitive. Keep alert notifications, dashboards, API snapshots, and incident notes limited to the teams that need access.

## General alert flow

1. Confirm the KBeacon Agent is healthy and Prometheus is scraping it.
2. Identify the affected cluster, namespace, Secret, workload, or cache resource from alert labels.
3. Use the Agent API or kbeaconctl to inspect current dependency state.
4. Check whether the alert maps to an expected rollout, Secret rotation, certificate renewal, or workload deployment.
5. Record impacted workloads and owner teams in the incident, change, or release ticket.

Useful baseline commands:

    kubectl -n kbeacon-system rollout status deploy/kbeacon
    kubectl -n kbeacon-system logs deploy/kbeacon --tail=100
    kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
    curl -sS http://127.0.0.1:8081/readyz | jq
    curl -sS http://127.0.0.1:8081/api/v1/config | jq

When the alert names a Secret, use:

    kbeaconctl impact report <namespace> <secret-name>

When reviewing before and after state, use:

    kbeaconctl snapshot export --output before.json
    kbeaconctl snapshot export --output after.json
    kbeaconctl snapshot diff --format markdown before.json after.json

## KBeaconAgentDown

### Meaning

Prometheus cannot scrape the KBeacon Agent target or the Agent is unavailable.

### Triage

1. Check the Deployment rollout and Pod status.
2. Check the Service and scrape target labels.
3. Review recent chart, image, RBAC, or node changes.

### Useful commands

    kubectl -n kbeacon-system get deploy,pod,svc -l app.kubernetes.io/name=kbeacon
    kubectl -n kbeacon-system describe deploy/kbeacon
    kubectl -n kbeacon-system logs deploy/kbeacon --tail=200
    kubectl -n kbeacon-system port-forward svc/kbeacon 8081:8080
    curl -sS http://127.0.0.1:8081/healthz | jq

### Resolution

Restore the Agent Deployment, Service, scrape configuration, or network path. Confirm Prometheus target health and that the alert clears.

## KBeaconCacheNotSynced

### Meaning

A Kubernetes informer cache has not reported synced state for a watched resource.

### Triage

1. Identify the resource label from the alert.
2. Check KBeacon logs for informer, RBAC, or API watch errors.
3. Confirm the rendered RBAC includes get, list, and watch for the enabled resource.

### Useful commands

    kubectl -n kbeacon-system logs deploy/kbeacon --tail=200
    kubectl auth can-i list pods --as=system:serviceaccount:kbeacon-system:kbeacon
    kubectl auth can-i list secrets --as=system:serviceaccount:kbeacon-system:kbeacon
    curl -sS http://127.0.0.1:8081/readyz | jq

### Resolution

Fix RBAC, API connectivity, or disabled watcher configuration. Confirm the readiness endpoint reports the expected cache as synced or optional.

## KBeaconGraphRebuildLatencyHigh

### Meaning

Dependency graph rebuild p95 latency is above the configured alert threshold.

### Triage

1. Check whether there was a workload or Secret churn spike.
2. Review cluster scale and edge metric cardinality.
3. Consider whether detailed edge metrics should be disabled in large clusters.

### Useful commands

    kubectl -n kbeacon-system logs deploy/kbeacon --tail=200
    curl -sS http://127.0.0.1:8081/api/v1/config | jq
    curl -sS http://127.0.0.1:8081/api/v1/dependency-map | jq ".data.edges | length"

### Resolution

Tune KBeacon resource limits, reduce unnecessary watched namespaces, disable detailed edge metrics when needed, or investigate unusual Kubernetes object churn.

## KBeaconSecretChangedWithImpact

### Meaning

A Secret changed recently and currently affects at least one workload.

### Triage

1. Identify whether the Secret change was planned.
2. Use the impact report to list affected workloads and teams.
3. Check affected workload rollout status and application alerts.

### Useful commands

    kbeaconctl impact report <namespace> <secret-name>
    kubectl -n <namespace> get secret <secret-name>
    kubectl get deploy,statefulset,daemonset,job,cronjob -A

### Resolution

If the change was planned, confirm affected workloads are healthy. If unplanned, coordinate rollback, Secret recreation, or workload restart according to the owning team process.

## KBeaconCriticalSecretChangedRecently

### Meaning

A high or critical Secret changed recently and affects workloads.

### Triage

1. Treat this as higher priority than a normal Secret change.
2. Identify owner team and impacted workloads.
3. Confirm whether the Secret rotation, certificate renewal, or credential change was approved.

### Useful commands

    kbeaconctl impact report <namespace> <secret-name>
    kubectl -n <namespace> get events --sort-by=.lastTimestamp
    kubectl -n <namespace> rollout status deploy/<workload-name>

### Resolution

Coordinate with the owner team, validate workload health, and record the impact assessment in the incident or change ticket.

## KBeaconHighImpactSecret

### Meaning

A Secret has an impact score above the configured high-impact threshold.

### Triage

1. Review affected workload, team, and namespace fan-out.
2. Confirm whether ownership and criticality metadata are accurate.
3. Consider additional review controls for future changes to this Secret.

### Useful commands

    kbeaconctl impact report <namespace> <secret-name>
    kbeaconctl get secrets --namespace <namespace>

### Resolution

Document ownership, classify the Secret correctly, and ensure planned changes include the affected teams.

## KBeaconLargeSecretFanout

### Meaning

A Secret affects many workloads.

### Triage

1. List affected workloads and namespaces.
2. Check whether the fan-out is expected, such as a shared registry credential or root certificate.
3. Validate owner-team metadata and change process.

### Useful commands

    kbeaconctl impact report <namespace> <secret-name>
    kbeaconctl get dependency-map --secret-name <secret-name>

### Resolution

Use a staged rollout or coordinated change plan for future updates. Split overly broad shared Secrets where appropriate.

## KBeaconUnresolvedSecretReference

### Meaning

A workload references a Secret that KBeacon cannot observe as existing.

### Triage

1. Confirm whether Secret watching is disabled for low-privilege mode.
2. Check whether the Secret name or namespace is wrong.
3. Check whether the reference is optional.

### Useful commands

    kbeaconctl get dependency-map --resolved false
    kubectl -n <namespace> get secret <secret-name>
    kubectl -n <namespace> describe deploy/<workload-name>

### Resolution

Create or fix the referenced Secret, correct the workload reference, or document the low-privilege expected behavior.

## KBeaconNoDependenciesDiscovered

### Meaning

KBeacon sees workloads but no Secret dependencies have been discovered.

### Triage

1. Confirm workloads actually reference Secrets.
2. Check namespace include and exclude filters.
3. Check discovery mode and disabled resource watchers.
4. Confirm KBeacon is watching Pods and workload controllers.

### Useful commands

    curl -sS http://127.0.0.1:8081/api/v1/config | jq
    curl -sS http://127.0.0.1:8081/api/v1/workloads | jq
    curl -sS http://127.0.0.1:8081/api/v1/dependency-map | jq ".data.edges | length"

### Resolution

Fix discovery mode, namespace filters, or resource watcher configuration. Confirm dependency edges appear in the API or metrics.
