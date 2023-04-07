# docker build -t bigip-kubernetes-gateway:latest -f build/Dockerfile .
FROM alpine:3.16.2

COPY ../cmd/controller/bigip-kubernetes-gateway-controller-linux /

RUN date > /build_date