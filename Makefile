ORG=jsanda
PROJECT=reaper-operator
REG=docker.io
SHELL=/bin/bash
TAG?=latest
PKG=github.com/jsanda/reaper-operator
COMPILE_TARGET=./build/_output/bin/$(PROJECT)

BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
REV=$(shell git rev-parse --short=12 HEAD)

IMAGE_BASE=$(REG)/$(ORG)/$(PROJECT)
PRE_TEST_TAG=$(BRANCH)-$(REV)-TEST
POST_TEST_TAG=$(BRANCH)-$(REV)
E2E_IMAGE?=$(IMAGE_BASE):$(PRE_TEST_TAG)
BRANCH_REV_IMAGE=$(IMAGE_BASE):$(POST_TEST_TAG)
REV_IMAGE=$(IMAGE_BASE):$(REV)
BRANCH_LATEST_IMAGE=$(IMAGE_BASE):$(BRANCH)-latest
LATEST_IMAGE=$(IMAGE_BASE):latest

ifeq ($(CIRCLE_BRANCH),master)
	PUSH_LATEST := true
endif

DEV_NS ?= reaper
E2E_NS?=reaper-e2e

.PHONY: clean
clean:
	rm -rf build/_output

.PHONY: e2e-image
e2e-image:
	@echo E2E_IMAGE = $(E2E_IMAGE)

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

.PHONY: build-e2e-image
build-e2e-image:
	@echo Building ${E2E_IMAGE}
	@operator-sdk build ${E2E_IMAGE}

.PHONY: push-e2e-image
push-e2e-image:
	docker push ${E2E_IMAGE}

.PHONY: build-image
build-image: code-gen openapi-gen
	@operator-sdk build ${REG}/${ORG}/${PROJECT}:${TAG}

.PHONY: push-image
push-image:
	@echo Pushing ${BRANCH_REV_IMAGE}
	docker tag ${E2E_IMAGE} ${BRANCH_REV_IMAGE}
	docker push ${BRANCH_REV_IMAGE}
	@echo Pushing ${REV_IMAGE}
	docker tag ${BRANCH_REV_IMAGE} ${REV_IMAGE}
	docker push ${REV_IMAGE}
ifdef CIRCLE_BRANCH
	@echo Pushing ${BRANCH_LATEST_IMAGE}
	docker tag ${BRANCH_REV_IMAGE} ${BRANCH_LATEST_IMAGE}
	docker push ${BRANCH_LATEST_IMAGE}
endif
ifdef PUSH_LATEST
	@echo PUSHING ${LATEST_IMAGE}
	docker tag ${BRANCH_REV_IMAGE} ${LATEST_IMAGE}
	docker push ${IMAGE_BASE}:latest
endif

.PHONY: unit-test
unit-test:
	@echo Running tests:
	go test -v -race -cover ./pkg/...

.PHONY: do-deploy-casskop
do-deploy-casskop:
	kubectl -n $(CASSKOP_NS) apply -f config/casskop

.PHONY: deploy-casskop
deploy-casskop: CASSKOP_NS ?= $(DEV_NS)
deploy-casskop: do-deploy-casskop

.PHONY: create-e2e-ns
create-e2e-ns:
	./scripts/create-ns.sh $(E2E_NS)

.PHONY: e2e-setup
e2e-setup: CASSKOP_NS = $(E2E_NS)
e2e-setup: create-e2e-ns do-deploy-casskop

.PHONY: e2e-test
e2e-test: e2e-setup
	@echo Running e2e tests
	operator-sdk test local ./test/e2e --image $(E2E_IMAGE) --namespace $(E2E_NS) --debug

.PHONY: e2e-test-local
e2e-test-local: e2e-setup
	@echo Running e2e tests
	operator-sdk test local ./test/e2e --up-local --namespace $(E2E_NS)