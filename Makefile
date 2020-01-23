ORG=jsanda
PROJECT=reaper-operator
REG=docker.io
SHELL=/bin/bash
TAG?=latest
PKG=github.com/jsanda/reaper-operator
COMPILE_TARGET=./build/_output/bin/$(PROJECT)

DEV_NS ?= reaper
E2E_NS?=reaper-e2e

.PHONY: clean
clean:
	rm -rf build/_output

.PHONY: run
run:
	@operator-sdk up local --namespace=${DEV_NS}

.PHONY: build
build:
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o=$(COMPILE_TARGET) ./cmd/manager

.PHONY: code-gen
code-gen:
	operator-sdk generate k8s

.PHONE: openapi-gen
openapi-gen:
	operator-sdk generate openapi

.PHONY: build-image
build-image: code-gen openapi-gen
	@operator-sdk build ${REG}/${ORG}/${PROJECT}:${TAG}

.PHONY: push-image
push-image:
	docker push ${REG}/${ORG}/${PROJECT}:${TAG}

.PHONY: unit-test
unit-test:
	@echo Running tests:
	go test -v -race -cover ./pkg/...

.PHONY: create-e2e-ns
create-e2e-ns:
	./scripts/create-ns.sh $(E2E_NS)

.PHONY: e2e-setup
e2e-setup: create-e2e-ns

.PHONY: e2e-test
e2e-test: e2e-setup
	@echo Running e2e tests
	operator-sdk test local ./test/e2e --namespace $(E2E_NS)
