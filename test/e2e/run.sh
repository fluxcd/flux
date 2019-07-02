#!/usr/bin/env bash

set -o errexit

declare -a on_exit_items

function on_exit() {
    if [ "${#on_exit_items[@]}" -gt 0 ]; then
        echo -e '\nRunning deferred items, please do not interrupt until they are done:'
    fi
    for I in "${on_exit_items[@]}"; do
        echo "deferred: ${I}"
        eval "${I}"
    done
}

# Cleaning up only makes sense in a local environment
# it just wastes time in CircleCI 
if [ "${CI}" != 'true' ]; then
    trap on_exit EXIT
fi

function defer() {
    on_exit_items=("$*" "${on_exit_items[@]}")
}

REPO_ROOT=$(git rev-parse --show-toplevel)
SCRIPT_DIR="${REPO_ROOT}/test/e2e"
KIND_VERSION="v0.4.0"
CACHE_DIR="${REPO_ROOT}/cache/$CURRENT_OS_ARCH"
KIND_CACHE_PATH="${CACHE_DIR}/kind-$KIND_VERSION"
KIND_CLUSTER=flux-e2e
USING_KIND=false
FLUX_NAMESPACE=flux-e2e
DEMO_NAMESPACE=demo


# Check if there is a kubernetes cluster running, otherwise use Kind
if ! kubectl version > /dev/null 2>&1 ; then
    if [ ! -f "${KIND_CACHE_PATH}" ]; then
        echo '>>> Downloading Kind'
        mkdir -p "${CACHE_DIR}"
        curl -sL "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-${CURRENT_OS_ARCH}" -o "${KIND_CACHE_PATH}"
    fi
    echo '>>> Creating Kind Kubernetes cluster'
    cp "${KIND_CACHE_PATH}" "${REPO_ROOT}/test/bin/kind"
    chmod +x "${REPO_ROOT}/test/bin/kind"
    defer kind --name "${KIND_CLUSTER}" delete cluster > /dev/null 2>&1
    kind create cluster --name "${KIND_CLUSTER}" --wait 5m
    export KUBECONFIG="$(kind --name="${KIND_CLUSTER}" get kubeconfig-path)"
    USING_KIND=true
    kubectl get pods --all-namespaces
fi


if ! helm version > /dev/null 2>&1; then
    echo '>>> Installing Tiller'
    kubectl --namespace kube-system create sa tiller
    defer kubectl --namespace kube-system delete sa tiller
    kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
    defer kubectl delete clusterrolebinding tiller-cluster-rule
    helm init --service-account tiller --upgrade --wait
    defer helm reset --force
fi

kubectl create namespace "$FLUX_NAMESPACE"
defer kubectl delete namespace "$FLUX_NAMESPACE"

echo '>>> Installing mock git server'
ssh-keygen -t rsa -N "" -f "${SCRIPT_DIR}/id_rsa"
defer rm -f "${SCRIPT_DIR}/id_rsa" "${SCRIPT_DIR}/id_rsa.pub"
kubectl create secret generic ssh-git --namespace="${FLUX_NAMESPACE}" --from-file="${SCRIPT_DIR}/known_hosts" --from-file="${SCRIPT_DIR}/id_rsa" --from-file=identity="${SCRIPT_DIR}/id_rsa" --from-file="${SCRIPT_DIR}/id_rsa.pub"
kubectl apply -n "${FLUX_NAMESPACE}" -f "${SCRIPT_DIR}/gitsrv.yaml"
kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/gitsrv


if [ "${USING_KIND}" = 'true' ]; then
    echo '>>> Loading images into the Kind cluster'
    kind --name "${KIND_CLUSTER}" load docker-image 'docker.io/fluxcd/flux:latest'
    kind --name "${KIND_CLUSTER}" load docker-image 'docker.io/fluxcd/helm-operator:latest'
fi

echo '>>> Installing Flux with Helm'

CREATE_CRDS='true'
if kubectl get crd fluxhelmreleases.helm.integrations.flux.weave.works helmreleases.flux.weave.works > /dev/null 2>&1; then
  # CRDs existed, don't try to create them
  echo 'CRDs existed, setting helmOperator.createCRD=false'
  CREATE_CRDS='false'
else
  # Schedule CRD deletion before calling helm, since it may fail and create them anyways
  defer kubectl delete crd fluxhelmreleases.helm.integrations.flux.weave.works helmreleases.flux.weave.works > /dev/null 2>&1
fi

KNOWN_HOSTS=$(cat "${REPO_ROOT}/test/e2e/known_hosts")
GITCONFIG=$(cat "${REPO_ROOT}/test/e2e/gitconfig")


defer helm delete --purge flux > /dev/null 2>&1

helm install --name flux --wait \
--namespace "${FLUX_NAMESPACE}" \
--set image.repository=docker.io/fluxcd/flux \
--set image.tag=latest \
--set git.url=ssh://git@gitsrv/git-server/repos/cluster.git \
--set git.secretName=ssh-git \
--set git.pollInterval=30s \
--set git.config.secretName=gitconfig \
--set git.config.enabled=true \
--set-string git.config.data="${GITCONFIG}" \
--set helmOperator.repository=docker.io/fluxcd/helm-operator \
--set helmOperator.tag=latest \
--set helmOperator.create=true \
--set helmOperator.createCRD=true \
--set helmOperator.git.secretName=ssh-git \
--set registry.excludeImage=* \
--set-string ssh.known_hosts="${KNOWN_HOSTS}" \
--set helmOperator.createCRD="${CREATE_CRDS}" \
"${REPO_ROOT}/chart/flux"




echo -n '>>> Waiting for gitconfig secret '
retries=24
count=0
ok=false
until ${ok}; do
    actual=$(kubectl get secrets -n "${FLUX_NAMESPACE}" gitconfig -ojsonpath={..data.gitconfig} | base64 --decode)
    if [ "${actual}" = "${GITCONFIG}" ]; then
        echo ' Expected Git configuration deployed'
        kubectl get secrets -n "${FLUX_NAMESPACE}" gitconfig && echo
        ok=true
    else
        echo -n  '.'
        ok=false
        sleep 5
    fi
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
        echo ' No more retries left'
        kubectl -n "${FLUX_NAMESPACE}" get secrets
        exit 1
    fi
done

echo -n ">>> Waiting for namespace ${DEMO_NAMESPACE} "
retries=24
count=1
ok=false
until ${ok}; do
    kubectl describe "ns/${DEMO_NAMESPACE}" && ok=true || ok=false
    echo -n '.'
    sleep 5
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
        kubectl -n "${FLUX_NAMESPACE}" logs deployment/flux
        echo ' No more retries left'
        exit 1
    fi
done
echo ' done'

echo -n '>>> Waiting for workload podinfo '
retries=24
count=0
ok=false
until ${ok}; do
    kubectl -n "${DEMO_NAMESPACE}" describe deployment/podinfo && ok=true || ok=false
    echo -n '.'
    sleep 5
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
        kubectl -n "${FLUX_NAMESPACE}" logs deployment/flux
        echo ' No more retries left'
        exit 1
    fi
done
echo ' done'

echo -n '>>> Waiting for Helm release mongodb '
retries=24
count=0
ok=false
until ${ok}; do
    kubectl -n $DEMO_NAMESPACE describe deployment/mongodb && ok=true || ok=false
    echo -n '.'
    sleep 5
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
        kubectl -n "${FLUX_NAMESPACE}" logs deployment/flux
        kubectl -n "${FLUX_NAMESPACE}" logs deployment/flux-helm-operator
        echo ' No more retries left'
        exit 1
    fi
done
echo ' done'

echo '>>> Flux logs'
kubectl -n "${FLUX_NAMESPACE}" logs deployment/flux

echo '>>> Helm Operator logs'
kubectl -n "${FLUX_NAMESPACE}" logs deployment/flux-helm-operator

echo '>>> List pods'
kubectl -n "${DEMO_NAMESPACE}" get pods

echo '>>> Check workload'
kubectl -n "${DEMO_NAMESPACE}" rollout status deployment/podinfo

echo '>>> Check Helm release'
kubectl -n "${DEMO_NAMESPACE}" rollout status deployment/mongodb

echo -e '\nEnd to end test was successful!!\n'
