SHELL := /usr/bin/env bash
GO ?= go
HELM ?= helm
IMAGE ?= ghcr.io/memoliyasti/kbeacon
TAG ?= dev
PROMETHEUS_IMAGE ?= prom/prometheus:v3.1.0


.PHONY: test
test:
	$(GO) test ./...

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: run
run:
	KBEACON_CLUSTER_NAME=local-dev $(GO) run ./cmd/kbeacon-agent

.PHONY: docker-build
docker-build:
	docker build -t $(IMAGE):$(TAG) .

.PHONY: helm-template
helm-template:
	$(HELM) template kbeacon ./charts/kbeacon --set cluster.name=local-dev

.PHONY: package
package:
	git archive --format=zip --output kbeacon.zip HEAD


.PHONY: helm-lint
helm-lint:
	$(HELM) lint ./charts/kbeacon --set cluster.name=local-dev

.PHONY: prom-rules
prom-rules:
	docker run --rm -i --entrypoint=promtool $(PROMETHEUS_IMAGE) check rules /dev/stdin < examples/prometheus/rules.yaml

.PHONY: docs
docs:
	python3 -m pip install -r requirements-docs.txt
	mkdocs build --strict

.PHONY: ci
ci: fmt test helm-lint helm-template helm-template-low-privilege prom-rules

.PHONY: helm-template-low-privilege
helm-template-low-privilege:
	$(HELM) template kbeacon ./charts/kbeacon --set cluster.name=local-dev --set resourcesToWatch.core.secrets=false > /tmp/kbeacon-low-privilege-rendered.yaml
	@if grep -n 'resources: \["secrets"\]' /tmp/kbeacon-low-privilege-rendered.yaml; then \
		echo "low-privilege render unexpectedly contains Secret RBAC"; \
		exit 1; \
	fi
	@grep -n 'secrets: false' /tmp/kbeacon-low-privilege-rendered.yaml
