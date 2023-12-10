#!/bin/bash

local_host_ipaddr=10.250.64.107
kube_config=/Users/zong/.kube/config

k="kubectl --kubeconfig $kube_config"

eval "cat <<EOF
$(< 0.prepare-certificates.yaml.tmpl)
EOF
" > prepare-certificate.yaml

$k apply -f prepare-certificate.yaml
if [ $? -ne 0 ]; then 
    echo "Error: Failed to create certificate"
    exit 1; 
fi

while true; do 
    $k get secret/webhook-server-cert -n kube-system; 
    if [ $? -eq 0 ]; then break; fi
    echo "waiting for secret webhook-server-cert ready"; sleep 1; 
done

$k get secret webhook-server-cert -n kube-system -o json | jq '.data["tls.crt"]' | tr -d '"' | base64 -d > certificates/tls.crt
$k get secret webhook-server-cert -n kube-system -o json | jq '.data["tls.key"]' | tr -d '"' | base64 -d > certificates/tls.key


eval "cat <<EOF
$(< 1.validating-webhook-configuration.yaml.tmpl)
EOF
" > validating-webhook-configuration.yaml

$k apply -f validating-webhook-configuration.yaml


eval "cat <<EOF
$(< 2.vscode-launch.json.tmpl)
EOF
" > launch.json

echo "Copy the launch.json to .vscode folder in the project root folder"