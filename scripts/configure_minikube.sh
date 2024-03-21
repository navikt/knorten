#!/usr/bin/env bash

set -e
source scripts/params.sh

kubernetes_replicator_version="v2.9.2"
gateway_api_httproutes_version="v1.0.0"
healthcheckpolicy_version="0e34ee15ce9ac6f6b96292a5291d2cd2097669da"

resources=(
  ./local/k8s/knaudit-proxy.yaml
  "https://raw.githubusercontent.com/mittwald/kubernetes-replicator/${kubernetes_replicator_version}/deploy/rbac.yaml"
  "https://raw.githubusercontent.com/mittwald/kubernetes-replicator/${kubernetes_replicator_version}/deploy/deployment.yaml"
  "https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/${gateway_api_httproutes_version}/config/crd/standard/gateway.networking.k8s.io_httproutes.yaml"
  "https://raw.githubusercontent.com/GoogleCloudPlatform/gke-networking-recipes/${healthcheckpolicy_version}/gateway-api/config/servicepolicies/crd/standard/healthcheckpolicy.yaml"
)

for resource in "${resources[@]}"; do
  kubectl apply --context "${kube_ctx}" -f "${resource}"
done

HELM_REPOSITORY_CONFIG="./.helm-repositories.yaml" helm repo update
HELM_REPOSITORY_CONFIG="./.helm-repositories.yaml" helm --kube-context "${kube_ctx}" upgrade --install cnpg --namespace cnpg-system --create-namespace cnpg/cloudnative-pg
