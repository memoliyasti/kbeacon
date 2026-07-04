#!/usr/bin/env bash
set -euo pipefail

page="docs/reference/supported-resources.md"

test -s "${page}"

required_terms=(
  "Secret"
  "Pod"
  "ServiceAccount"
  "Deployment"
  "StatefulSet"
  "DaemonSet"
  "Job"
  "CronJob"
  "Ingress"
  "resourcesToWatch.core.secrets"
  "resourcesToWatch.core.pods"
  "resourcesToWatch.core.serviceAccounts"
  "resourcesToWatch.apps.deployments"
  "resourcesToWatch.apps.statefulSets"
  "resourcesToWatch.apps.daemonSets"
  "resourcesToWatch.batch.jobs"
  "resourcesToWatch.batch.cronJobs"
  "resourcesToWatch.networking.ingresses"
  "resourcesToWatch.certManager.certificates"
  "env.secretKeyRef"
  "envFrom.secretRef"
  "volumes.secret"
  "volumes.projected.sources.secret"
  "imagePullSecrets"
  "serviceAccount.imagePullSecrets"
  "ingress.tls"
  "cert-manager.certificate.spec.secretName"
  "annotation"
  "cert-manager Certificate"
  "ExternalSecret"
  "SecretProviderClass"
  "Strimzi KafkaConnector"
  "Confluent Connector"
)

for term in "${required_terms[@]}"
do
  if ! grep -Fq "${term}" "${page}"
  then
    echo "ERROR: supported resource matrix missing term: ${term}"
    exit 1
  fi
done

grep -Fq "Supported resources: reference/supported-resources.md" mkdocs.yml
grep -Fq "Supported resources" README.md
grep -Fq "supported-resources.md" docs/reference/helm.md
grep -Fq "supported-resources.md" charts/kbeacon/README.md
grep -Fq "supported-resources.md" docs/user-guide/discovery-modes.md

echo "supported resource matrix validation passed"
