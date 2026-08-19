VERSION        ?= 0.0.666
IMAGE_NAME     := "quay.io/snowdrop/cert-manager-webhook-godaddy"
IMAGE_TAG      := "latest"
TEST_ZONE_NAME ?= example.com.

OUT := $(shell pwd)/_out

$(shell mkdir -p "$(OUT)")

clean:
	rm -rf vendor
	chmod -R u+w $(OUT) 2>/dev/null || true
	rm -rf $(OUT)
	rm -rf apiserver.local.config

install-tools:
	sh ./scripts/fetch-test-binaries.sh

test-v1:
	go test -v ./internal/godaddy/v1/

test-v3:
	go test -v ./internal/godaddy/v3/

test-unit:
	go test -v ./internal/...

verify: clean install-tools
	go test -v .

TEST_API_VERSION ?= v1

test: clean install-tools
	TEST_ASSET_ETCD=$(OUT)/kubebuilder/bin/etcd \
	TEST_ASSET_KUBECTL=$(OUT)/kubebuilder/bin/kubectl \
	TEST_ASSET_KUBE_APISERVER=$(OUT)/kubebuilder/bin/kube-apiserver \
	TEST_ZONE_NAME=$(TEST_ZONE_NAME) \
	TEST_DNS_SERVER=$(TEST_DNS_SERVER) \
	TEST_API_VERSION=$(TEST_API_VERSION) \
	TEST_TIMEOUT=$(TEST_TIMEOUT) go test .

compile:
	echo "### Go mod vendor ..."
	go mod vendor
	echo "### Compile the webhook ..."
	CGO_ENABLED=0 go build -o webhook -ldflags '-w -extldflags "-static"' .

build:
	docker build -t "$(IMAGE_NAME):$(IMAGE_TAG)" .

push:
	docker push "$(IMAGE_NAME):$(IMAGE_TAG)"

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml:
	helm template \
	    --name godaddy-webhook \
        --set image.repository=$(IMAGE_NAME) \
        --set image.tag=$(IMAGE_TAG) \
        deploy/godaddy-webhook > "$(OUT)/rendered-manifest.yaml"

version:
	@echo $(VERSION)
