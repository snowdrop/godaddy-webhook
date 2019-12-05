IMAGE_NAME := "quay.io/snowdrop/cert-manager-webhook-godaddy"
IMAGE_TAG  := "latest"

OUT := $(shell pwd)/_out

$(shell mkdir -p "$(OUT)")

verify:
	go test -v .

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
