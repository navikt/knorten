#!/usr/bin/env bash

email=$(gcloud config get-value account)

if [ -z "$email" ]; then
  echo "Email address is not set and cannot be found in gcloud config."
  exit 1
fi

server="https://europe-north1-docker.pkg.dev"
project="knada-gcp"
repository="knada-north"
location="europe-north1"
secret_name="gcp-auth"
kube_ctx="minikube"
prod_ctx="gke_knada-gcp_europe-north1_knada-gke"

find_namespace() {
  local namespaces
  local count

  namespaces=$(kubectl --context="${kube_ctx}" get namespaces --no-headers=true | awk '{print $1}' | grep '^team-')
  count=$(echo "${namespaces}" | wc -l)

  if [ "${count}" -eq 1 ]; then
    echo "${namespaces}"
  elif [ "${count}" -gt 1 ]; then
    echo "More than one 'team-' namespace found." >&2
    exit 1
  else
    echo "No 'team-' namespace found." >&2
    exit 1
  fi
}

namespace=$(find_namespace)
