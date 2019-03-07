#!/usr/bin/env bash

set -o errexit

REPO_ROOT=$(git rev-parse --show-toplevel)

echo ">>> Installing kubectl"
curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
chmod +x kubectl && \
sudo mv kubectl /usr/local/bin/

echo ">>> Building sigs.k8s.io/kind"
docker build -t kind:src . -f ${REPO_ROOT}/test/e2e/Dockerfile.kind
docker create -ti --name dummy kind:src sh
docker cp dummy:/go/bin/kind ./kind
docker rm -f dummy

echo ">>> Installing kind"
chmod +x kind
sudo mv kind /usr/local/bin/
kind create cluster --wait 5m

export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"
kubectl get pods --all-namespaces
