.SHELLFLAGS = -e -c
SHELL := /bin/bash
# Current Operator version
VERSION ?= 0.0.1
# Default bundle image tag
BUNDLE_IMG ?= controller-bundle:$(VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

ORG?=k8ssandra
PROJECT=reaper-operator
REG?=docker.io

BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
REV=$(shell git rev-parse --short=8 HEAD)

IMAGE_BASE=$(REG)/$(ORG)/$(PROJECT)
REV_IMAGE=$(IMAGE_BASE):$(REV)
LATEST_IMAGE=$(IMAGE_BASE):latest

# Image URL to use all building/pushing image targets
IMG ?= $(LATEST_IMAGE)

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

KREW_INSTALLED=$(shell kubectl krew; $?)
KUTTL_KIND_CFG ?= "./test/config/kind/w1k1.21.yaml"

all: manager

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin/bin
test: generate fmt vet manifests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.3/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh && fetch_envtest_tools $(ENVTEST_ASSETS_DIR) && setup_envtest_env $(ENVTEST_ASSETS_DIR) && go test -v ./controllers/... ./pkg/... 
	# -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# E2E tests from kuttl
kuttl-test: install-kuttl
	PATH="./testbin/bin:$$PATH"; kubectl-kuttl test --kind-config=${KUTTL_KIND_CFG}

 # Install kuttl for e2e tests.
install-kuttl: 
	mkdir -p ./testbin/bin ; \
	cd ./testbin/bin ; \
	OS="$$(uname | tr '[:upper:]' '[:lower:]')" ; \
  	ARCH="$$(uname -m | sed -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$$/arm64/')" ; \
	curl -LO https://github.com/kudobuilder/kuttl/releases/download/v0.11.1/kuttl_0.11.1_$${OS}_$${ARCH}.tar.gz ; \
	tar -zxvf kuttl_0.11.1_$${OS}_$${ARCH}.tar.gz ; 

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=reaper-operator webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

PHONY: init-test-overlays
init-test-overlays:
	./scripts/create-test-overlays.sh

PHONY: e2e-test
e2e-test:
	@echo Running e2e tests
	go test -timeout 1800s ./test/e2e/... -ginkgo.v -ginkgo.progress -test.v

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build:
	@echo Building ${REV_IMAGE}
	docker build -t ${REV_IMAGE} .
	docker tag ${REV_IMAGE} ${LATEST_IMAGE}

# Push the docker image
docker-push:
	docker push ${REV_IMAGE}
	docker push ${LATEST_IMAGE}

kind-load-image:
	kind load docker-image ${LATEST_IMAGE}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.5.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .
