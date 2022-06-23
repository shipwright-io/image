IMGCTRL = imgctrl
PLUGIN = kubectl-image
PLUGIN_DARWIN = kubectl-image-darwin

VERSION ?= v0.0.0

REGISTRY_HOSTNAME ?= ghcr.io
REGISTRY_USERNAME ?= shipwright-io

IMAGE_BUILDER ?= podman
IMAGE_TAG ?= latest
IMAGE ?= $(REGISTRY_HOSTNAME)/$(REGISTRY_USERNAME)/$(IMGCTRL)

OUTPUT_DIR ?= output
OUTPUT_BIN = $(OUTPUT_DIR)/bin

IMGCTRL_BIN = $(OUTPUT_BIN)/$(IMGCTRL)
PLUGIN_BIN = $(OUTPUT_BIN)/$(PLUGIN)

# destination namespace to install target
NAMESPACE ?= shipwright-build

# the container image produced by ko will use this repostory, and combine with the application name
# being compiled
KO_DOCKER_REPO ?= $(REGISTRY_HOSTNAME)/$(REGISTRY_USERNAME)

# golang flags are exported through the enviroment variables, reaching all targets
GOFLAGS ?= -v -mod=vendor -ldflags='-Xmain.Version=$(VERSION)'

.EXPORT_ALL_VARIABLES:

default: build

# build target calls builds for the operator and for the kubectl plugin for the platform we
# are running on plus Darwin amd64.
.PHONY: build
build: $(IMGCTRL) $(PLUGIN_DARWIN) $(PLUGIN)

.PHONY: $(IMGCTRL)
$(IMGCTRL):
	go build -o $(IMGCTRL_BIN) ./cmd/$(IMGCTRL)

.PHONY: $(PLUGIN)
$(PLUGIN):
	go build -o $(PLUGIN_BIN) ./cmd/$(PLUGIN)

.PHONY: $(PLUGIN_DARWIN)
$(PLUGIN_DARWIN):
	GOOS=darwin GOARCH=amd64 \
	     go build -tags containers_image_openpgp -o $(PLUGIN_BIN) ./cmd/$(PLUGIN)

# get target download a bunch of binaries that are necessary while developing. Things like
# binaries to generate code, generate crds and run our e2e tests are installed when this
# target is called.
.PHONY: get
get: get-code-generator get-kuttl get-controller-gen get-proto

# this target generates the protobuf, the kubernetes clients code and the necessary CRDs as
# yaml files. The latter are installed under chart/templates directory.
.PHONY: generate
generate: generate-proto generate-k8s generate-manifests

.PHONY: get-code-generator
get-code-generator:
	./hack/get-code-generator.sh

.PHONY: get-controller-gen
get-controller-gen:
	./hack/get-controller-gen.sh

.PHONY: get-kuttl
get-kuttl:
	./hack/get-kuttl.sh

.PHONY: get-proto
get-proto:
	./hack/get-proto.sh

.PHONY: e2e
e2e:
	output/kuttl/kuttl test --timeout=180 e2e

.PHONY: generate-proto
generate-proto:
	output/protoc/protoc --go-grpc_out=paths=source_relative:. \
		--go_out=paths=source_relative:. \
		./infra/pb/*.proto

.PHONY: generate-k8s
generate-k8s:
	./hack/generate-k8s.sh

.PHONY: generate-manifests
generate-manifests:
	./hack/generate-manifests.sh

.PHONY: image
image:
	$(IMAGE_BUILDER) build --build-arg version=$(VERSION) -f Containerfile -t $(IMAGE) .

.PHONY: clean
clean:
	rm -rf $(OUTPUT_DIR)

.PHONY: pdf
pdf:
	./hack/generate-pdf.sh

# using the environment variable directly to login against the container registry, while the
# username and container registry hostname are regular Makefile variables, please, overwrite
# them as needed.
registry-login:
	echo "$$GITHUB_TOKEN" | \
		ko login "$(REGISTRY_HOSTNAME)" --username=$(REGISTRY_USERNAME) --password-stdin

# build and push the container image with ko.
build-image:
	ko publish --base-import-paths --tags="${IMAGE_TAG}" ./cmd/$(IMGCTRL) 

# installs the helm rendered resources against the infomred namespace.
install:
	helm template \
		--namespace="$(NAMESPACE)" \
		--set="image=ko://github.com/shipwright-io/image/cmd/imgctrl" \
		./chart | \
			ko apply --base-import-paths --filename -
