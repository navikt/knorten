#!/usr/bin/env bash

set -e

kubectx=$1
namespace=$2

if [[ -z $kubectx ]] || [[ -z $namespace ]]; then
  echo "Usage: $0 <kubectx> <namespace>"
  exit 1
fi

# Confirmation prompt
read -p "You are about to switch Airflow database in Kubernetes cluster '$kubectx', and namespace '$namespace'. Do you want to continue? (yes/no): " confirm
confirm=$(echo $confirm | tr '[:upper:]' '[:lower:]')
if [[ $confirm =~ ^(yes|y)$ ]]; then
    echo "Proceeding with operations..."
else
    echo "Operation cancelled."
    exit 1
fi

env_file="migration-backup/$kubectx/$namespace/env"

if [[ -f $env_file ]]; then
  set -a
  source $env_file
  set +a

  envsubst < "templates/secret-template.yaml" > "$secret_template_output"
  envsubst < "templates/cluster-template.yaml" > "$cluster_template_output"

  echo "Templates rendered for namespace '$namespace'"
else
  echo "Env file $env_file does not exist"
fi

namespace_name=$(basename "$namespace")
team_name=${namespace_name#team-}

new_uri=$(kubectl --context $kubectx get secret "${team_name}-app" --namespace "$namespace" -o json | jq -r '.data.uri | @base64d')

rendered_dir="migration-backup/$kubectx/$namespace/rendered"
airflow_secret_template_output="$rendered_dir/airflow-secret.yaml"

set -a
export URI=$new_uri
set +a

envsubst < "templates/airflow-secret-template.yaml" > "$airflow_secret_template_output"

if [[ -f $airflow_secret_template_output ]]; then
  echo "Updating airlow secret new resources for namespace '$namespace'..."
  kubectl --context $kubectx apply -f "$airflow_secret_template_output"

  kubectl --context $kubectx scale deployment airflow-webserver --namespace $namespace --replicas=2
  kubectl --context $kubectx scale deployment airflow-scheduler --namespace $namespace --replicas=2

  echo "Resources applied for namespace '$namespace'"
else
  echo "Templates for namespace '$namespace' do not exist"
fi
