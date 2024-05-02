#!/usr/bin/env bash

set -e

kubectx=$1

if [[ -z $kubectx ]]; then
  echo "Usage: $0 <kubectx>"
  exit 1
fi

./backup_secrets.sh "$kubectx"
./populate_env_files.sh
./populate_templates.sh
