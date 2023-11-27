all: upload

timestamp ?= $(shell date +%Y%m%d-%H%M%S)
version ?= latest
controller_docker_repo ?= zongzw/bigip-kubernetes-gateway-controller
webhook_docker_repo ?= zongzw/bigip-kubernetes-gateway-webhook

controller_image_name := ${controller_docker_repo}:${version}-${timestamp}
webhook_image_name := ${webhook_docker_repo}:${version}-${timestamp}

upload: upload_controller upload_webhook

upload_controller: controller_image
	docker push ${controller_image_name}

upload_webhook: webhook_image
	docker push ${webhook_image_name}

image: controller_image webhook_image

controller_image: controller_binary
	cd .. && docker buildx build --no-cache -t ${controller_image_name} -f build/Dockerfile-controller .

webhook_image: webhook_binary
	cd .. && docker buildx build --no-cache -t ${webhook_image_name} -f build/Dockerfile-webhook .

binary: controller_binary webhook_binary

controller_binary:
	cd ../cmd/controller; \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -ldflags '-s -w --extldflags "-static -fpic"' -o ../../build/bigip-kubernetes-gateway-controller-linux; \
	# CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
	# go build -ldflags '-s -w --extldflags "-static -fpic"' -o ../../build/bigip-kubernetes-gateway-controller-darwin

webhook_binary:
	cd ../cmd/webhook; \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -ldflags '-s -w --extldflags "-static -fpic"' -o ../../build/bigip-kubernetes-gateway-webhook-linux; 