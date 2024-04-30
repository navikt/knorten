#!/usr/bin/env bash

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

kubectx=$1

# Confirmation prompt
echo -e "You are about to run operations against the Kubernetes cluster ${GREEN}'$kubectx'${NC}."
read -p "Do you want to continue? (yes/no): " confirm
confirm=$(echo $confirm | tr '[:upper:]' '[:lower:]')
if [[ $confirm =~ ^(yes|y)$ ]]; then
    echo -e "${GREEN}Proceeding with operations...${NC}"
else
    echo -e "${RED}Operation cancelled.${NC}"
    exit 1
fi

backup_dir="migration-backup/$kubectx"
mkdir -p "$backup_dir"

namespaces=$(kubectl --context $kubectx get namespaces --no-headers | awk '{print $1}' | grep '^team-')

for namespace in $namespaces; do
  echo "Processing namespace '$namespace'"

  crd_instance=$(kubectl --context $kubectx get clusters.postgresql.cnpg.io --namespace "$namespace" --no-headers 2>/dev/null | tr -d '[:space:]')

  if [ -n "$crd_instance" ]; then
    echo -e "${YELLOW}Skipping backup for namespace '$namespace' as it has a CRD instance of type clusters.postgresql.cnpg.io${NC}"
    continue
  fi

  secret=$(kubectl --context $kubectx get secret "airflow-db" --namespace "$namespace" 2>/dev/null)
  if [ -n "$secret" ]; then
      # Create a directory specific for the namespace
      ns_backup_dir="$backup_dir/$namespace"
      mkdir -p "$ns_backup_dir"

      # Export the secret in yaml format
      kubectl --context $kubectx get secret "airflow-db" --namespace "$namespace" -o yaml > "$ns_backup_dir/airflow-db-secret.yaml"
      echo -e "${GREEN}Backup up secret 'airflow-db' in namespace '$namespace' to '$ns_backup_dir/airflow-db-secret.yaml'${NC}"
  else
      echo -e "${YELLOW}Secret 'airflow-db' not found in namespace '$namespace'${NC}"
  fi
done

echo -e "${GREEN}Backup process completed.${NC}"
