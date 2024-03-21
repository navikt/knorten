#!/usr/bin/env bash

set -e
source scripts/params.sh

copy_k8s_secret_with_replicate() {
  local secret_name=$1
  local to_secret_name=$2
  local src_ctx=$3
  local src_ns=$4
  local target_ctx=$5
  local target_ns=$6
  local replicate_to_pattern=$7

  kubectl get secret "$secret_name" --context "$src_ctx" --namespace "$src_ns" -o json | \
        jq 'del(.metadata.creationTimestamp,.metadata.resourceVersion,.metadata.selfLink,.metadata.uid,.metadata.namespace) |
            .metadata.annotations["replicator.v1.mittwald.de/replicate-to"] = "'"$replicate_to_pattern"'" |
            .metadata.name = "'"$to_secret_name"'"' | \
          kubectl apply --context "$target_ctx" --namespace "$target_ns" -f -
}

copy_k8s_configmap_with_replicate() {
  local cm_name=$1
  local src_ctx=$2
  local src_ns=$3
  local target_ctx=$4
  local target_ns=$5
  local replicate_to_pattern=$6

  kubectl get configmap "$cm_name" --context "$src_ctx" --namespace "$src_ns" -o json | \
    jq 'del(.metadata.creationTimestamp,.metadata.resourceVersion,.metadata.selfLink,.metadata.uid,.metadata.namespace) |
        .metadata.annotations["replicator.v1.mittwald.de/replicate-to"] = "'"$replicate_to_pattern"'"' | \
      kubectl apply --context "$target_ctx" --namespace "$target_ns" -f -
}

copy_k8s_secret_with_replicate github-app-secret github-secret "${prod_ctx}" knada-system "${kube_ctx}" kube-system "team-.*"
copy_k8s_configmap_with_replicate airflow-webserver-cm "${prod_ctx}" team-nada-oqs1 "${kube_ctx}" kube-system "team-.*"
copy_k8s_configmap_with_replicate airflow-auth-cm "${prod_ctx}" team-nada-oqs1 "${kube_ctx}" kube-system "team-.*"
copy_k8s_configmap_with_replicate ca-bundle-pem "${prod_ctx}" team-nada-oqs1 "${kube_ctx}" kube-system "team-.*"
