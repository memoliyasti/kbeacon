SHELL := /usr/bin/env bash
GO ?= go
HELM ?= helm
IMAGE ?= ghcr.io/kbeacon/kbeacon-agent
TAG ?= dev

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
