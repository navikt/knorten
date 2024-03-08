#!/usr/bin/env bash

set -e
source scripts/params.sh

kubectl apply --context "${kube_ctx}" -f ./local/k8s/knada-proxy.yaml
kubectl apply --context "${kube_ctx}" -f https://raw.githubusercontent.com/mittwald/kubernetes-replicator/master/deploy/rbac.yaml
kubectl apply --context "${kube_ctx}" -f https://raw.githubusercontent.com/mittwald/kubernetes-replicator/master/deploy/deployment.yaml
kubectl apply --context "${kube_ctx}" -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/main/config/crd/standard/gateway.networking.k8s.io_httproutes.yaml
kubectl apply --context "${kube_ctx}" -f https://raw.githubusercontent.com/GoogleCloudPlatform/gke-networking-recipes/main/gateway-api/config/servicepolicies/crd/standard/healthcheckpolicy.yaml
HELM_REPOSITORY_CONFIG="./.helm-repositories.yaml" helm repo update
HELM_REPOSITORY_CONFIG="./.helm-repositories.yaml" helm --kube-context "${kube_ctx}" upgrade --install cnpg --namespace cnpg-system --create-namespace cnpg/cloudnative-pg
