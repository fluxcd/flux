#!/usr/bin/env bash

set -o errexit

REPO_ROOT=$(git rev-parse --show-toplevel)
export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"

KNOWN_HOSTS=$(cat ${REPO_ROOT}/test/e2e/known_hosts)

echo ">>> Installing Flux with Helm"
helm install --name flux --wait \
--namespace flux \
--set git.url=ssh://git@gitsrv/git-server/repos/cluster.git \
--set git.secretName=ssh-git \
--set git.pollInterval=30s \
--set helmOperator.create=true \
--set helmOperator.createCRD=true \
--set helmOperator.git.secretName=ssh-git \
--set registry.excludeImage=* \
--set-string ssh.known_hosts="${KNOWN_HOSTS}" \
${REPO_ROOT}/chart/flux

#TODO: replace this will long pooling
sleep 120

echo ">>> Flux logs"
kubectl -n flux logs deployment/flux

echo ">>> Helm Operator logs"
kubectl -n flux logs deployment/flux-helm-operator

echo ">>> List pods"
kubectl -n test get pods

echo ">>> Check workload"
kubectl -n test rollout status deployment/podinfo

echo ">>> Check Helm release"
kubectl -n test rollout status deployment/mongodb