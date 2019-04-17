#!/usr/bin/env bash

set -o errexit

export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"
REPO_ROOT=$(git rev-parse --show-toplevel)
KNOWN_HOSTS=$(cat ${REPO_ROOT}/test/e2e/known_hosts)

echo ">>> Loading $(docker/image-tag) into the cluster"
kind load docker-image "docker.io/weaveworks/flux:$(docker/image-tag)"
kind load docker-image "docker.io/weaveworks/helm-operator:$(docker/image-tag)"

echo ">>> Installing Flux with Helm"
helm install --name flux --wait \
--namespace flux \
--set image.tag=$(docker/image-tag) \
--set git.url=ssh://git@gitsrv/git-server/repos/cluster.git \
--set git.secretName=ssh-git \
--set git.pollInterval=30s \
--set helmOperator.tag=$(docker/image-tag) \
--set helmOperator.create=true \
--set helmOperator.createCRD=true \
--set helmOperator.git.secretName=ssh-git \
--set registry.excludeImage=* \
--set-string ssh.known_hosts="${KNOWN_HOSTS}" \
${REPO_ROOT}/chart/flux

echo '>>> Waiting for namespace demo'
retries=12
count=0
ok=false
until ${ok}; do
    kubectl describe ns/demo && ok=true || ok=false
    sleep 10
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
        kubectl -n flux logs deployment/flux
        echo "No more retries left"
        exit 1
    fi
done

echo '>>> Waiting for workload podinfo'
retries=12
count=0
ok=false
until ${ok}; do
    kubectl -n demo describe deployment/podinfo && ok=true || ok=false
    sleep 10
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
        kubectl -n flux logs deployment/flux
        echo "No more retries left"
        exit 1
    fi
done

echo '>>> Waiting for Helm release mongodb'
retries=12
count=0
ok=false
until ${ok}; do
    kubectl -n demo describe deployment/mongodb && ok=true || ok=false
    sleep 10
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
        kubectl -n flux logs deployment/flux
        kubectl -n flux logs deployment/flux-helm-operator
        echo "No more retries left"
        exit 1
    fi
done

echo ">>> Flux logs"
kubectl -n flux logs deployment/flux

echo ">>> Helm Operator logs"
kubectl -n flux logs deployment/flux-helm-operator

echo ">>> List pods"
kubectl -n demo get pods

echo ">>> Check workload"
kubectl -n demo rollout status deployment/podinfo

echo ">>> Check Helm release"
kubectl -n demo rollout status deployment/mongodb
