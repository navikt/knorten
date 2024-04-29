#!/usr/bin/env bash

set -e

kubectx=$1
namespace=$2

if [[ -z $kubectx ]] || [[ -z $namespace ]]; then
  echo "Usage: $0 <kubectx> <namespace>"
  exit 1
fi

# Confirmation prompt
read -p "You are about to apply new resources against the Kubernetes cluster '$kubectx', and namespace '$namespace'. Do you want to continue? (yes/no): " confirm
confirm=$(echo $confirm | tr '[:upper:]' '[:lower:]')
if [[ $confirm =~ ^(yes|y)$ ]]; then
    echo "Proceeding with operations..."
else
    echo "Operation cancelled."
    exit 1
fi

rendered_dir="migration-backup/$kubectx/$namespace/rendered"
secret_template_output="$rendered_dir/secret.yaml"
cluster_template_output="$rendered_dir/cluster.yaml"

if [[ -f $secret_template_output && -f $cluster_template_output ]]; then
  echo "Scaling down the Airflow webserver and scheduler deployment to 0 replicas..."
  kubectl --context $kubectx scale deployment airflow-webserver --namespace $namespace --replicas=0
  kubectl --context $kubectx scale deployment airflow-scheduler --namespace $namespace --replicas=0

  echo "Applying new resources for namespace '$namespace'..."
  kubectl --context $kubectx apply -f "$secret_template_output"
  kubectl --context $kubectx apply -f "$cluster_template_output"

  echo "Resources applied for namespace '$namespace'"
else
  echo "Templates for namespace '$namespace' do not exist"
fi
