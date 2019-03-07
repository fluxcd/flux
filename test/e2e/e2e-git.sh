#!/usr/bin/env bash

set -o errexit

REPO_ROOT=$(git rev-parse --show-toplevel)
export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"

echo ">>> Installing git"
kubectl create namespace flux
kubectl apply -f ${REPO_ROOT}/test/e2e/ssh-key.yaml
kubectl apply -f ${REPO_ROOT}/test/e2e/git-dep.yaml

kubectl -n flux rollout status deployment/gitsrv

