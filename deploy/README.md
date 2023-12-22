# Installation Guide

Use the yaml ordered with `<number>` for bigip-kubernetes-gateway installation.

---

Note:

There are two images that cannot be pulled directly in somecase, if so, process them separately in some manual steps:

* Use `docker save & docker load` to setup the mentioned images for docker env.

* Use `ctr -n k8s.io image export & import` to setup the mentioned images for containerd env.

  *The images’ format is universal: docker save -> ctr -n k8s.io image import*

### For v0.5.1
```shell
k8s.gcr.io/ingress-nginx/kube-webhook-certgen:v1.1.1
gcr.io/k8s-staging-gateway-api/admission-server:v0.5.1
```

### For v0.6.0
```shell
k8s.gcr.io/ingress-nginx/kube-webhook-certgen:v1.1.1
gcr.io/k8s-staging-gateway-api/admission-server:v0.6.0
```

### For v1.0.0
```shell
registry.k8s.io/ingress-nginx/kube-webhook-certgen:v1.1.1
registry.k8s.io/gateway-api/admission-server:v1.0.0
```
