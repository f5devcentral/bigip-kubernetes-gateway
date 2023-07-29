These two images cannot be downloaded in China and need to be processed separately.

# for 0.5.1
k8s.gcr.io/ingress-nginx/kube-webhook-certgen:v1.1.1
gcr.io/k8s-staging-gateway-api/admission-server:v0.5.1

# for 0.6.0
k8s.gcr.io/ingress-nginx/kube-webhook-certgen:v1.1.1
gcr.io/k8s-staging-gateway-api/admission-server:v0.6.0

Use `docker save & docker load` to setup the mentioned images for docker env.
Use `ctr -n k8s.io image export & import` to setup the mentioned images for containerd env.
*The images’ format is universal: docker save -> ctr -n k8s.io image import*