#!/usr/bin/env bash

# Install a git server into the cluster.

set -o errexit

source $(dirname $0)/e2e-paths.env
source $(dirname $0)/e2e-kube.env

SCRIPT_DIR=$REPO_ROOT/test/e2e

echo ">>> Installing git"
kubectl create namespace flux
ssh-keygen -t rsa -N "" -f "${SCRIPT_DIR}/id_rsa"
kubectl create secret generic ssh-git --namespace=flux --from-file="${SCRIPT_DIR}/known_hosts" --from-file="${SCRIPT_DIR}/id_rsa" --from-file=identity="${SCRIPT_DIR}/id_rsa" --from-file="${SCRIPT_DIR}/id_rsa.pub"
rm "${SCRIPT_DIR}/id_rsa" "${SCRIPT_DIR}/id_rsa.pub"
kubectl apply -f "${SCRIPT_DIR}/git-dep.yaml"

kubectl -n flux rollout status deployment/gitsrv
