#!/usr/bin/env bash

set -e
source scripts/params.sh

check_role() {
  local role_present

  role_present=$(gcloud artifacts repositories get-iam-policy "${repository}" \
    --location="${location}" \
    --project="${project}" \
    --format="table(bindings.role, bindings.members)" \
    | grep 'roles/artifactregistry.reader' \
    | grep "user:${email}")

  echo "${role_present}"
}

assign_role() {
  gcloud artifacts repositories add-iam-policy-binding "${repository}" \
    --location="${location}" \
    --project="${project}" \
    --member="user:${email}" \
    --role="roles/artifactregistry.reader"
}

delete_secret() {
  kubectl --namespace="${namespace}" delete secret "${secret_name}" --ignore-not-found
}

create_secret() {
  kubectl --namespace="${namespace}" create secret docker-registry ${secret_name} \
    --docker-server="${server}" \
    --docker-username="oauth2accesstoken" \
    --docker-password="$(gcloud auth print-access-token)" \
    --docker-email="${email}"
}

role_present=$(check_role)
if [ -n "${role_present}" ]; then
  echo "User '${email}' already has roles/artifactregistry.reader role."
else
  echo "Adding roles/artifactregistry.reader role to user '${email}'"
  if ! assign_role;
  then
    echo "Failed to assign the role."
    exit 1
  fi
fi

echo "Ensuring the '${secret_name}' secret is up to date."
delete_secret
create_secret
