all: image_upload

timestamp ?= $(shell date +%Y%m%d-%H%M%S)
version ?= latest
docker_repo ?= zongzw/bigip-kubernetes-gateway

image_name := ${docker_repo}:${version}-${timestamp}

image_upload: image_build
	docker push ${image_name}

image_build: binary_build
	cd .. && docker build \
		--no-cache \
		-t ${image_name} \
		-f build/Dockerfile .

binary_build:
	cd ../cmd/gateway; \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -ldflags '-s -w --extldflags "-static -fpic"' -o bigip-kubernetes-gateway-controller-linux; \
	# CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
	# go build -ldflags '-s -w --extldflags "-static -fpic"' -o bigip-kubernetes-gateway-controller-darwin
