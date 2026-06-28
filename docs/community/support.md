# Support

KBeacon is an open source project maintained on a best-effort basis.

## Where to ask

Use GitHub Issues for:

- reproducible bugs;
- documentation problems;
- feature requests;
- Helm chart problems;
- metric or API contract questions.

Use GitHub Discussions if enabled for:

- design questions;
- usage questions;
- integration ideas;
- roadmap discussion.

Security issues must follow SECURITY.md.

## What to include

For bugs, include:

- KBeacon version;
- Kubernetes version;
- installation method;
- relevant Helm values;
- Agent logs;
- readyz output;
- relevant Prometheus query output;
- minimal workload and Secret manifests that reproduce the issue.

Do not include real Secret values, production tokens, kubeconfigs, or customer identifiers.

## Response expectations

There is no commercial SLA. Maintainers prioritize:

1. security issues;
2. correctness bugs that may mislead operators;
3. release or installation breakages;
4. documentation fixes;
5. feature requests.

For production incidents, use your organization's incident response process. KBeacon maintainers do not operate user clusters.
