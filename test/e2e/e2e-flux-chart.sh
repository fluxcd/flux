#!/usr/bin/env bash

set -o errexit

REPO_ROOT=$(git rev-parse --show-toplevel)
export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"

echo ">>> Installing Flux with Helm"
helm install --name flux --wait \
--namespace flux \
--set git.url=ssh://git@gitsrv/git-server/repos/cluster.git \
--set git.secretName=ssh-git \
--set helmOperator.create=true \
--set helmOperator.createCRD=true \
--set helmOperator.git.secretName=ssh-git \
${REPO_ROOT}/chart/flux

sleep 30

kubectl -n flux logs deployment/flux
kubectl -n flux logs deployment/flux-helm-operator
