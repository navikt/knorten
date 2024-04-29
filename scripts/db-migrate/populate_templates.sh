#!/usr/bin/env bash

set -e

dir="migration-backup"

for cluster in "$dir"/*; do
  for namespace in "$cluster"/*; do
    mkdir -p "$namespace/rendered"

    env_file="$namespace/env"
    secret_template_output="$namespace/rendered/secret.yaml"
    cluster_template_output="$namespace/rendered/cluster.yaml"

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
  done
done
