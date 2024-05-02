#!/usr/bin/env bash

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

set -e

dir="migration-backup"

for cluster in "$dir"/*; do
  for namespace in "$cluster"/*; do
    secret_file="$namespace/airflow-db-secret.yaml"

    if [[ -f $secret_file ]]; then
      connection=$(grep 'connection:' "$secret_file" | awk '{print $2}')

      decoded_connection=$(echo "$connection" | base64 --decode)

      # Split the PostgreSQL connection string into its different parts
      protocol=${decoded_connection%%://*}
      userinfo_host_port_db=${decoded_connection#*://}
      userinfo=${userinfo_host_port_db%%@*}
      username=${userinfo%%:*}
      password=${userinfo#*:}
      host_port_db=${userinfo_host_port_db#*@}
      host_port=${host_port_db%%/*}
      host=${host_port%%:*}
      port=${host_port#*:}
      db_sslmode=${host_port_db#*/}
      database=${db_sslmode%%\?*}
      sslmode=${db_sslmode#*?sslmode=}

      # Other
      namespace_name=$(basename "$namespace")
      team_name=${namespace_name#team-}

      # If sslmode is not present, it will be the same as database, so set it to an empty string
      if [[ $sslmode == $database ]]; then
        sslmode=""
      else
        sslmode="?sslmode=$sslmode"
      fi

      env_file="$namespace/env"

      encoded_password=$(echo -n "$password" | base64)

      echo "PROTOCOL=\"$protocol\"" > "$env_file"
      echo "USERNAME=\"$username\"" >> "$env_file"
      echo "PASSWORD=\"$encoded_password\"" >> "$env_file"
      echo "HOST=\"$host\"" >> "$env_file"
      echo "PORT=\"$port\"" >> "$env_file"
      echo "DATABASE=\"$database\"" >> "$env_file"
      echo "SSLMODE=\"$sslmode\"" >> "$env_file"
      echo "NAMESPACE=\"$namespace_name\"" >> "$env_file"
      echo "TEAM_NAME=\"$team_name\"" >> "$env_file"

      echo "Environment file created for namespace '$namespace'"

    else
      echo "${YELLOW}Secret file $secret_file does not exist${NC}"
    fi
  done
done
