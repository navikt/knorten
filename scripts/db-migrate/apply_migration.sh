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

echo -e "You are about to apply new resources against the Kubernetes cluster ${GREEN}'$kubectx'${NC}, and namespace ${GREEN}'$namespace'${NC}."
read -r -p "Do you want to continue? (yes/no): " confirm
confirm=$(echo "$confirm" | tr '[:upper:]' '[:lower:]')
if [[ $confirm =~ ^(yes|y)$ ]]; then
    echo -e "Proceeding with operations..."
else
    echo -e "${RED}Operation cancelled.${NC}"
    exit 1
fi

rendered_dir="migration-backup/$kubectx/$namespace/rendered"
secret_template_output="$rendered_dir/secret.yaml"
cluster_template_output="$rendered_dir/cluster.yaml"

if [[ -f $secret_template_output && -f $cluster_template_output ]]; then
  echo -e "${GREEN}Scaling down the Airflow webserver and scheduler deployment to 0 replicas...${NC}"
  kubectl --context "$kubectx" scale deployment airflow-webserver --namespace "$namespace" --replicas=0
  kubectl --context "$kubectx" scale deployment airflow-scheduler --namespace "$namespace" --replicas=0

  echo -e "${GREEN}Applying new resources for namespace '$namespace'...${NC}"
  kubectl --context "$kubectx" apply -f "$secret_template_output"
  kubectl --context "$kubectx" apply -f "$cluster_template_output"

  echo -e "${GREEN}Resources applied for namespace '$namespace'${NC}"
else
  echo -e "${YELLOW}Templates for namespace '$namespace' do not exist${NC}"
fi
