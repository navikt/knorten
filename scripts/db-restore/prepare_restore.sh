#!/usr/bin/env bash

set -e

kubectx=$1
namespace=$2

if [[ -z $kubectx ]] || [[ -z $namespace ]]; then
  echo "Usage: $0 <kubectx> <namespace>"
  exit 1
fi

./fetch_cluster.sh "$kubectx" "$namespace"
./populate_templates.sh "$kubectx" "$namespace"
