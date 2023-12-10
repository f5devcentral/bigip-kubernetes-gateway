This guide aims to make it clear how to develop bigip-kubernetes-gateway-webhook program.

Refer to `setup-webhook-dev.sh` for more details.

Basically, it setup 3 things for developing webhook program:

* create the webhook server crt/key via cert-manager.io, see `0.prepare-cerfitifcate.yaml.tmpl` for detail.

* create the webhook validating configuration, see `1.validating-webhook-configuration.yaml.tmpl` for detail.

* create the vscode `launch.json` for debugging.

During the process, variables are needed:

* `local_host_ipaddr`: the callback IP address for webhook API.

* `kube_config`: the kubeconfig file for accessing kubernetes API.