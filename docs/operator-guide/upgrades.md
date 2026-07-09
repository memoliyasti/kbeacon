# Upgrade and rollback runbook

This runbook describes safe KBeacon upgrades and rollbacks with Helm.

## Pre-upgrade checks

```bash
helm list -n kbeacon-system
helm repo update kbeacon-release
helm search repo kbeacon-release/kbeacon --versions | head -10
kbeacon --namespace kbeacon-system ready
kbeacon --namespace kbeacon-system get config
kbeacon --namespace kbeacon-system snapshot export --output kbeacon-before.json
```

## Upgrade

```bash
VERSION=0.3.19

helm upgrade --install kbeacon kbeacon-release/kbeacon \
  --version "${VERSION}" \
  --namespace kbeacon-system \
  --create-namespace \
  --reuse-values \
  --wait \
  --timeout 10m

kubectl -n kbeacon-system rollout status deploy/kbeacon --timeout=5m
kbeacon --namespace kbeacon-system ready
kbeacon --namespace kbeacon-system get config
```

## Post-upgrade snapshot diff

```bash
kbeacon --namespace kbeacon-system snapshot export --output kbeacon-after.json
kbeacon snapshot diff --format markdown kbeacon-before.json kbeacon-after.json
```

## Rollback

```bash
helm history kbeacon -n kbeacon-system
helm rollback kbeacon <REVISION> -n kbeacon-system --wait --timeout 10m
kubectl -n kbeacon-system rollout status deploy/kbeacon --timeout=5m
kbeacon --namespace kbeacon-system ready
```

## Public artifact verification

```bash
hack/verify-release-artifacts.sh v0.3.19
```
