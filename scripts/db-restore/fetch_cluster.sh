#!/usr/bin/env bash

set -e
# Uncomment for more verbose output:
# set -x

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

kubectx=$1
namespace=$2

# Confirmation prompt
echo -e "You are about to run operations against the Kubernetes cluster ${GREEN}'$kubectx'${NC} and namespace ${GREEN}'$namespace'${NC}"
read -p "Do you want to continue? (yes/no): " confirm
confirm=$(echo "$confirm" | tr '[:upper:]' '[:lower:]')
if [[ $confirm =~ ^(yes|y)$ ]]; then
    echo -e "${GREEN}Proceeding with operations...${NC}"
else
    echo -e "${RED}Operation cancelled.${NC}"
    exit 1
fi

backup_dir="restore-staging/$kubectx/$namespace"
mkdir -p "$backup_dir"

crd_instance=$(kubectl --context "$kubectx" get clusters.postgresql.cnpg.io --namespace "$namespace" --no-headers 2>/dev/null | awk '{print $1}' | tr -d '[:space:]')

if [ -z "$crd_instance" ]; then
  echo -e "${RED}Could not find a CRD instance of type clusters.postgresql.cnpg.io, nothing to do here.${NC}"
fi

kubectl --context "$kubectx" get clusters.postgresql.cnpg.io "$crd_instance" --namespace "$namespace" -o yaml > "$backup_dir/cluster_definition.yaml"

echo -e "${GREEN}Available backups for cluster (${crd_instance}), including failed:${NC}"
kubectl --context "$kubectx" get backups.postgresql.cnpg.io --namespace "$namespace" --selector=cnpg.io/cluster=$crd_instance

backup_definition=$(kubectl --context $kubectx get backups.postgresql.cnpg.io --namespace "$namespace" -o=jsonpath='{range .items[?(@.status.phase=="completed")]}{"\n"}{.metadata.name}{": creationTimestamp="}{.metadata.creationTimestamp}{end}' | sort -r -k3 | head -n 1 | cut -d ':' -f 1)

if [ -z "$backup_definition" ]; then
  echo -e "${RED}Could not find a Backup definition of type backups.postgresql.cnpg.io with label cnpg.io/cluster=$crd_instance.${NC}"
fi

echo -e "${GREEN}Selected backup${NC}: $backup_definition (should be latest completed)"

kubectl --context "$kubectx" get backups.postgresql.cnpg.io "$backup_definition" --namespace "$namespace" -o yaml > "$backup_dir/backup_definition.yaml"

echo -e "${GREEN}Staging of resources completed.${NC}"

echo "NAMESPACE=$namespace" > "$backup_dir/env"
echo "CLUSTER_NAME=$crd_instance" >> "$backup_dir/env"
echo "BACKUP_NAME=$backup_definition" >> "$backup_dir/env"
echo "OWNER=$crd_instance" >> "$backup_dir/env"
echo "DATABASE=airflow-$crd_instance" >> "$backup_dir/env"

echo -e "${GREEN}Environment variables written to: ${NC}$backup_dir/env"
