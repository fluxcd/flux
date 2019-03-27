#!/usr/bin/env bash

set -o errexit

REPO_ROOT=$(git rev-parse --show-toplevel)
SCRIPT_DIR="${REPO_ROOT}/test/e2e"
export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"

echo ">>> Installing git"
kubectl create namespace flux
ssh-keygen -t rsa -N "" -f "${SCRIPT_DIR}/id_rsa"
kubectl create secret generic ssh-git --namespace=flux --from-file="${SCRIPT_DIR}/known_hosts" --from-file="${SCRIPT_DIR}/id_rsa" --from-file=identity="${SCRIPT_DIR}/id_rsa" --from-file="${SCRIPT_DIR}/id_rsa.pub"
rm "${SCRIPT_DIR}/id_rsa" "${SCRIPT_DIR}/id_rsa.pub"
kubectl apply -f "${SCRIPT_DIR}/git-dep.yaml"

kubectl -n flux rollout status deployment/gitsrv

