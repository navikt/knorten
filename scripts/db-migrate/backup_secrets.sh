#!/usr/bin/env bash

set -e

kubectx=$1

# Confirmation prompt
read -p "You are about to run operations against the Kubernetes cluster '$kubectx'. Do you want to continue? (yes/no): " confirm
confirm=$(echo $confirm | tr '[:upper:]' '[:lower:]')
if [[ $confirm =~ ^(yes|y)$ ]]; then
    echo "Proceeding with operations..."
else
    echo "Operation cancelled."
    exit 1
fi

backup_dir="migration-backup/$kubectx"
mkdir -p "$backup_dir"

namespaces=$(kubectl --context $kubectx get namespaces --no-headers | awk '{print $1}' | grep '^team-')

for namespace in $namespaces; do
  echo "Processing namespace '$namespace'"

  secret=$(kubectl --context $kubectx get secret "airflow-db" --namespace "$namespace" 2>/dev/null)
  if [ -n "$secret" ]; then
      # Create a directory specific for the namespace
      ns_backup_dir="$backup_dir/$namespace"
      mkdir -p "$ns_backup_dir"

      # Export the secret in yaml format
      kubectl --context $kubectx get secret "airflow-db" --namespace "$namespace" -o yaml > "$ns_backup_dir/airflow-db-secret.yaml"
      echo "Backup up secret 'airflow-db' in namespace '$namespace' to '$ns_backup_dir/airflow-db-secret.yaml'"
  else
      echo "Secret 'airflow-db' not found in namespace '$namespace'"
  fi
done

echo "Backup process completed."
