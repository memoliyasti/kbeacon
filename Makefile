SHELL := /usr/bin/env bash
GO ?= go
HELM ?= helm
KIND ?= kind
KUBECTL ?= kubectl
PYTHON ?= python3
MKDOCS ?= mkdocs
IMAGE ?= ghcr.io/memoliyasti/kbeacon
TAG ?= dev
PROMETHEUS_IMAGE ?= prom/prometheus:v3.1.0
CLUSTER_NAME ?= ci
NAMESPACE ?= kbeacon-system
CHART_VERSION := $(shell awk '/^version:/ {print $$2; exit}' charts/kbeacon/Chart.yaml)

.PHONY: validate validate-ci ci fmt test api-contract-lint supply-chain-lint build run docker-build helm-lint helm-schema-lint helm-template helm-template-low-privilege helm-template-serviceaccount-disabled helm-template-ingress-disabled helm-template-networkpolicy helm-template-privacy-redaction helm-template-edge-disabled helm-template-prometheus-annotations helm-template-namespace prom-rules docs demo-lint demo-dry-run demo-metrics-live scale-generate scale-lint scale-dry-run scale-benchmark-lint scale-benchmark scale-delete stale-check release-metadata-check package clean dashboards-lint kind-e2e-smoke-lint kind-e2e-smoke supported-resources-lint ctl-build ctl-smoke runbooks-lint

validate: validate-ci demo-dry-run

validate-ci: fmt test api-contract-lint supply-chain-lint supported-resources-lint runbooks-lint build helm-lint helm-schema-lint helm-template helm-template-low-privilege helm-template-serviceaccount-disabled helm-template-ingress-disabled helm-template-networkpolicy helm-template-privacy-redaction helm-template-edge-disabled helm-template-prometheus-annotations helm-template-namespace prom-rules docs dashboards-lint demo-lint scale-lint scale-benchmark-lint stale-check release-metadata-check kind-e2e-smoke-lint ctl-smoke

ci: validate-ci

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

supply-chain-lint:
	grep -q "actions/attest@v4" .github/workflows/release.yaml
	grep -q "Generate release SBOMs" .github/workflows/release.yaml
	grep -q "kbeaconctl_" .github/workflows/release.yaml
	grep -q "./cmd/kbeaconctl" .github/workflows/release.yaml
	grep -q "subject-checksums: dist/checksums.txt" .github/workflows/release.yaml
	grep -q "provenance: true" .github/workflows/release.yaml
	grep -q "sbom: true" .github/workflows/release.yaml
	grep -q "SBOM" RELEASE.md docs/operator-guide/releases.md docs/operator-guide/security.md
	grep -q "attestation" RELEASE.md docs/operator-guide/releases.md docs/operator-guide/security.md

api-contract-lint:
	$(GO) test ./internal/server -run 'Test(OpenAPIContract|APIExampleContracts|HandlerResponsesMatchAPIContractShapes)'


ctl-build:
	$(GO) build -trimpath -o ./bin/kbeaconctl ./cmd/kbeaconctl

ctl-smoke: ctl-build
	./bin/kbeaconctl version >/tmp/kbeaconctl-version.txt
	grep -q "kbeaconctl version=" /tmp/kbeaconctl-version.txt

build:
	$(GO) build -trimpath -o ./bin/kbeacon-agent ./cmd/kbeacon-agent

run:
	KBEACON_CLUSTER_NAME=local-dev $(GO) run ./cmd/kbeacon-agent

docker-build:
	docker build -t $(IMAGE):$(TAG) .

helm-lint:
	$(HELM) lint ./charts/kbeacon --set cluster.name=$(CLUSTER_NAME)

helm-schema-lint:
	$(HELM) lint ./charts/kbeacon --set cluster.name=$(CLUSTER_NAME)
	! $(HELM) lint ./charts/kbeacon --set cluster.name=$(CLUSTER_NAME) --set discovery.defaultMode=invalid >/tmp/kbeacon-schema-invalid-mode.txt 2>&1
	grep -q "discovery/defaultMode" /tmp/kbeacon-schema-invalid-mode.txt
	! $(HELM) lint ./charts/kbeacon --set cluster.name=$(CLUSTER_NAME) --set log.level=verbose >/tmp/kbeacon-schema-invalid-log.txt 2>&1
	grep -q "log/level" /tmp/kbeacon-schema-invalid-log.txt
	! $(HELM) lint ./charts/kbeacon >/tmp/kbeacon-schema-missing-cluster.txt 2>&1
	grep -q "cluster/name" /tmp/kbeacon-schema-missing-cluster.txt

	! $(HELM) lint ./charts/kbeacon --set cluster.name=$(CLUSTER_NAME) --set service.type=ExternalName >/tmp/kbeacon-schema-invalid-service-type.txt 2>&1
	grep -Eq "service/type|service.type" /tmp/kbeacon-schema-invalid-service-type.txt
	! $(HELM) lint ./charts/kbeacon --set cluster.name=$(CLUSTER_NAME) --set networkPolicy.ingress.from=invalid >/tmp/kbeacon-schema-invalid-networkpolicy-from.txt 2>&1
	grep -Eq "networkPolicy/ingress/from|networkPolicy.ingress.from" /tmp/kbeacon-schema-invalid-networkpolicy-from.txt
	! $(HELM) lint ./charts/kbeacon --set cluster.name=$(CLUSTER_NAME) --set replicaCount=2 >/tmp/kbeacon-schema-invalid-replica-count.txt 2>&1
	grep -Eq "replicaCount|one of|single-replica|multi-replica" /tmp/kbeacon-schema-invalid-replica-count.txt

helm-template:
	$(HELM) template kbeacon ./charts/kbeacon --namespace $(NAMESPACE) --set cluster.name=$(CLUSTER_NAME) --set dashboards.enabled=true > /tmp/kbeacon-rendered.yaml

helm-template-low-privilege:
	$(HELM) template kbeacon ./charts/kbeacon --namespace $(NAMESPACE) --set cluster.name=$(CLUSTER_NAME) --set resourcesToWatch.core.secrets=false > /tmp/kbeacon-low-privilege-rendered.yaml
	! grep -q "resources: \[\"secrets\"\]" /tmp/kbeacon-low-privilege-rendered.yaml


helm-template-serviceaccount-disabled:
	$(HELM) template kbeacon ./charts/kbeacon --namespace $(NAMESPACE) --set cluster.name=$(CLUSTER_NAME) --set resourcesToWatch.core.serviceAccounts=false > /tmp/kbeacon-serviceaccount-disabled-rendered.yaml
	! grep -Fq 'resources: ["serviceaccounts"]' /tmp/kbeacon-serviceaccount-disabled-rendered.yaml
helm-template-ingress-disabled:
	$(HELM) template kbeacon ./charts/kbeacon --namespace $(NAMESPACE) --set cluster.name=$(CLUSTER_NAME) --set resourcesToWatch.networking.ingresses=false > /tmp/kbeacon-ingress-disabled-rendered.yaml
	! grep -Fq "resources: [\"ingresses\"]" /tmp/kbeacon-ingress-disabled-rendered.yaml


helm-template-networkpolicy:
	$(HELM) template kbeacon ./charts/kbeacon --namespace $(NAMESPACE) --set cluster.name=$(CLUSTER_NAME) --set networkPolicy.enabled=true --set 'networkPolicy.ingress.from[0].podSelector.matchLabels.app=prometheus' > /tmp/kbeacon-networkpolicy-rendered.yaml
	grep -q "kind: NetworkPolicy" /tmp/kbeacon-networkpolicy-rendered.yaml
	grep -q "podSelector:" /tmp/kbeacon-networkpolicy-rendered.yaml
	grep -q "app: prometheus" /tmp/kbeacon-networkpolicy-rendered.yaml


helm-template-privacy-redaction:
	$(HELM) template kbeacon ./charts/kbeacon --namespace $(NAMESPACE) --set cluster.name=$(CLUSTER_NAME) --set privacy.redaction.secretKeys=true > /tmp/kbeacon-privacy-redaction-rendered.yaml
	grep -q "secretKeys: true" /tmp/kbeacon-privacy-redaction-rendered.yaml

helm-template-edge-disabled:
	$(HELM) template kbeacon ./charts/kbeacon --namespace $(NAMESPACE) --set cluster.name=$(CLUSTER_NAME) --set metrics.edge.enabled=false > /tmp/kbeacon-edge-disabled-rendered.yaml


helm-template-prometheus-annotations:
	$(HELM) template kbeacon ./charts/kbeacon --namespace kbeacon-system --set cluster.name=ci --set prometheus.scrapeAnnotations.enabled=true > /tmp/kbeacon-prometheus-annotations-rendered.yaml
	grep -q "prometheus.io/scrape: \"true\"" /tmp/kbeacon-prometheus-annotations-rendered.yaml
	grep -q "prometheus.io/path: \"/metrics\"" /tmp/kbeacon-prometheus-annotations-rendered.yaml
	grep -q "prometheus.io/port: \"8080\"" /tmp/kbeacon-prometheus-annotations-rendered.yaml

helm-template-namespace:
	$(HELM) template kbeacon ./charts/kbeacon --namespace payments --set cluster.name=$(CLUSTER_NAME) --set rbac.scope=namespace --set-string discovery.namespaces.include[0]=payments > /tmp/kbeacon-namespace-rendered.yaml
	grep -q "kind: Role" /tmp/kbeacon-namespace-rendered.yaml
	! grep -q "kind: ClusterRole" /tmp/kbeacon-namespace-rendered.yaml

prom-rules:
	docker run --rm -i --entrypoint=promtool $(PROMETHEUS_IMAGE) check rules /dev/stdin < examples/prometheus/rules.yaml

docs:
	$(PYTHON) -m venv .venv-docs
	. .venv-docs/bin/activate && python -m pip install --upgrade pip && python -m pip install -r requirements-docs.txt && mkdocs build --strict

demo-lint:
	bash -n examples/demo-blast-radius/run.sh
	bash -n hack/validate-demo-metrics.sh

demo-dry-run: demo-lint
	kubectl apply --dry-run=client --validate=false -f examples/demo-blast-radius/namespace.yaml > /tmp/kbeacon-demo-dry-run.txt
	kubectl apply --dry-run=client --validate=false -f examples/demo-blast-radius/secrets.yaml >> /tmp/kbeacon-demo-dry-run.txt
	kubectl apply --dry-run=client --validate=false -f examples/demo-blast-radius/workloads.yaml >> /tmp/kbeacon-demo-dry-run.txt

demo-metrics-live: demo-lint
	./hack/validate-demo-metrics.sh

scale-generate:
	./hack/generate-scale-fixture.sh /tmp/kbeacon-scale-fixture kbeacon-scale 25 100

scale-lint:
	bash -n hack/generate-scale-fixture.sh
	./hack/generate-scale-fixture.sh /tmp/kbeacon-scale-fixture kbeacon-scale 5 10
	test -s /tmp/kbeacon-scale-fixture/namespace.yaml
	test -s /tmp/kbeacon-scale-fixture/secrets.yaml
	test -s /tmp/kbeacon-scale-fixture/workloads.yaml
	test -s /tmp/kbeacon-scale-fixture/expected-summary.json

scale-dry-run: scale-generate
	kubectl apply --dry-run=client --validate=false -f /tmp/kbeacon-scale-fixture/namespace.yaml > /tmp/kbeacon-scale-dry-run.txt
	kubectl apply --dry-run=client --validate=false -f /tmp/kbeacon-scale-fixture/secrets.yaml >> /tmp/kbeacon-scale-dry-run.txt
	kubectl apply --dry-run=client --validate=false -f /tmp/kbeacon-scale-fixture/workloads.yaml >> /tmp/kbeacon-scale-dry-run.txt

scale-benchmark-lint:
	bash -n hack/benchmark-scale.sh

scale-benchmark: scale-benchmark-lint
	./hack/benchmark-scale.sh

scale-delete:
	kubectl delete namespace kbeacon-scale --ignore-not-found

dashboards-lint:
	./hack/validate-dashboards.sh

stale-check:
	./hack/stale-check.sh

release-metadata-check:
	grep -q "version: $(CHART_VERSION)" charts/kbeacon/Chart.yaml
	grep -q "appVersion:.*$(CHART_VERSION)" charts/kbeacon/Chart.yaml
	grep -q "tag:.*$(CHART_VERSION)" charts/kbeacon/values.yaml
	grep -q "version: $(CHART_VERSION)" docs/api/openapi.yaml

package:
	git archive --format=zip --output kbeacon.zip HEAD

clean:
	rm -rf bin dist site .venv-docs kbeacon.zip
	./hack/validate-api-parity.sh

kind-e2e-smoke-lint:
	bash -n hack/e2e-kind-smoke.sh

kind-e2e-smoke: kind-e2e-smoke-lint
	KIND=$(KIND) KUBECTL=$(KUBECTL) HELM=$(HELM) ./hack/e2e-kind-smoke.sh

supported-resources-lint:
	./hack/validate-supported-resources.sh

runbooks-lint:
	./hack/validate-runbooks.sh
