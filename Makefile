SHELL := /usr/bin/env bash
GO ?= go
HELM ?= helm
PYTHON ?= python3
MKDOCS ?= mkdocs
IMAGE ?= ghcr.io/memoliyasti/kbeacon
TAG ?= dev
PROMETHEUS_IMAGE ?= prom/prometheus:v3.1.0
CLUSTER_NAME ?= ci
NAMESPACE ?= kbeacon-system
CHART_VERSION := $(shell awk '/^version:/ {print $$2; exit}' charts/kbeacon/Chart.yaml)

.PHONY: validate validate-ci ci fmt test build run docker-build helm-lint helm-template helm-template-low-privilege helm-template-edge-disabled helm-template-namespace prom-rules docs demo-lint demo-dry-run demo-metrics-live scale-generate scale-lint scale-dry-run scale-delete stale-check release-metadata-check package clean dashboards-lint

validate: validate-ci demo-dry-run

validate-ci: fmt test build helm-lint helm-template helm-template-low-privilege helm-template-edge-disabled helm-template-namespace prom-rules docs dashboards-lint demo-lint scale-lint stale-check release-metadata-check

ci: validate-ci

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

build:
	$(GO) build -trimpath -o ./bin/kbeacon-agent ./cmd/kbeacon-agent

run:
	KBEACON_CLUSTER_NAME=local-dev $(GO) run ./cmd/kbeacon-agent

docker-build:
	docker build -t $(IMAGE):$(TAG) .

helm-lint:
	$(HELM) lint ./charts/kbeacon --set cluster.name=$(CLUSTER_NAME)

helm-template:
	$(HELM) template kbeacon ./charts/kbeacon --namespace $(NAMESPACE) --set cluster.name=$(CLUSTER_NAME) --set dashboards.enabled=true > /tmp/kbeacon-rendered.yaml

helm-template-low-privilege:
	$(HELM) template kbeacon ./charts/kbeacon --namespace $(NAMESPACE) --set cluster.name=$(CLUSTER_NAME) --set resourcesToWatch.core.secrets=false > /tmp/kbeacon-low-privilege-rendered.yaml
	! grep -q "resources: \[\"secrets\"\]" /tmp/kbeacon-low-privilege-rendered.yaml

helm-template-edge-disabled:
	$(HELM) template kbeacon ./charts/kbeacon --namespace $(NAMESPACE) --set cluster.name=$(CLUSTER_NAME) --set metrics.edge.enabled=false > /tmp/kbeacon-edge-disabled-rendered.yaml

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
