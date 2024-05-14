#!/usr/bin/env bash

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

kubectx=$1
namespace=$2

dir="restore-staging/$kubectx/$namespace"

mkdir -p "$dir/rendered"

env_file="$dir/env"
cluster_template_output="$dir/rendered/cluster.yaml"
cluster_clean_template_output="$dir/rendered/cluster-clean.yaml"

if [[ -f $env_file ]]; then
  set -a
  source "$env_file"
  set +a

  envsubst < "templates/cluster-template.yaml" > "$cluster_template_output"
  echo -e "Templates rendered for ${GREEN}'$cluster_template_output'${NC}"

  envsubst < "templates/cluster-clean-template.yaml" > "$cluster_clean_template_output"
  echo -e "Templates rendered for ${GREEN}'$cluster_clean_template_output'${NC}"
else
  echo -e "${RED}Env file $env_file does not exist${NC}"
fi
