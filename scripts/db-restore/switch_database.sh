#!/usr/bin/env bash

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

kubectx=$1
namespace=$2

if [[ -z $kubectx ]] || [[ -z $namespace ]]; then
  echo "Usage: $0 <kubectx> <namespace>"
  exit 1
fi

# Confirmation prompt
echo -e "You are about to switch Airflow database in Kubernetes cluster ${GREEN}'$kubectx'${NC}, and namespace ${GREEN}'$namespace'${NC}."
read -r -p "Do you want to continue? (yes/no): " confirm
confirm=$(echo "$confirm" | tr '[:upper:]' '[:lower:]')
if [[ $confirm =~ ^(yes|y)$ ]]; then
    echo -e "${GREEN}Proceeding with operations...${NV}"
else
    echo -e "${RED}Operation cancelled.${NC}"
    exit 1
fi

team_name=${namespace#team-}

new_uri=$(kubectl --context "$kubectx" get secret "${team_name}-app" --namespace "$namespace" -o json | jq -r '.data.uri')

rendered_dir="restore-staging/$kubectx/$namespace/rendered"
airflow_secret_template_output="$rendered_dir/airflow-secret.yaml"

set -a
export URI=$new_uri
export NAMESPACE=$namespace
set +a

envsubst < "templates/airflow-db-template.yaml" > "$airflow_secret_template_output"

if [[ -f $airflow_secret_template_output ]]; then
  echo -e "${GREEN}Updating airlow secret new resources for namespace '$namespace'...${NC}"
  kubectl --context "$kubectx" apply -f "$airflow_secret_template_output"

  kubectl --context "$kubectx" scale deployment airflow-webserver --namespace "$namespace" --replicas=2
  kubectl --context "$kubectx" scale deployment airflow-scheduler --namespace "$namespace" --replicas=2

  echo -e "${GREEN}Resources applied for namespace '$namespace'${NC}"
else
  echo -e "${YELLOW}Templates for namespace '$namespace' do not exist${NC}"
fi
