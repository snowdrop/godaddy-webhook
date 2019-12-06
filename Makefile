IMAGE_NAME     := "quay.io/snowdrop/cert-manager-webhook-godaddy"
IMAGE_TAG      := "latest"
TEST_ZONE_NAME ?= example.com.

OUT := $(shell pwd)/_out

$(shell mkdir -p "$(OUT)")

verify:
	sh ./scripts/fetch-test-binaries.sh
	go test -v .

test:
	TEST_ZONE_NAME=$(TEST_ZONE_NAME) go test .

compile:
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
