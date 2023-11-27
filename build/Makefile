all: upload

timestamp ?= $(shell date +%Y%m%d-%H%M%S)
version ?= latest
controller_docker_repo ?= zongzw/bigip-kubernetes-gateway-controller

controller_image_name := ${controller_docker_repo}:${version}-${timestamp}

upload: images
	docker push ${controller_image_name}

images: binaries
	cd .. && docker build \
		--no-cache \
		-t ${controller_image_name} \
		-f build/Dockerfile .

binaries: controller

controller:
	cd ../cmd/controller; \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -ldflags '-s -w --extldflags "-static -fpic"' -o bigip-kubernetes-gateway-controller-linux; \
	# CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
	# go build -ldflags '-s -w --extldflags "-static -fpic"' -o bigip-kubernetes-gateway-controller-darwin
