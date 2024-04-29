#!/usr/bin/env bash

set -e

kubectx=$1
namespace=$2

if [[ -z $kubectx ]] || [[ -z $namespace ]]; then
  echo "Usage: $0 <kubectx> <namespace>"
  exit 1
fi

echo "Scaling up the Airflow webserver and scheduler deployment to 2 replicas..."
kubectl --context $kubectx scale deployment airflow-webserver --namespace $namespace --replicas=2
kubectl --context $kubectx scale deployment airflow-scheduler --namespace $namespace --replicas=2
