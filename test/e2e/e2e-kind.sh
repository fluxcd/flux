#!/usr/bin/env bash

set -o errexit

export GOPATH=$HOME/go
export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin
REPO_ROOT=$(git rev-parse --show-toplevel)

echo ">>> Installing kubectl"
curl -sLO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
chmod +x kubectl && \
sudo mv kubectl /usr/local/bin/

echo ">>> Building sigs.k8s.io/kind"
go get -u sigs.k8s.io/kind

echo ">>> Installing kind"
sudo cp $GOPATH/bin/kind /usr/local/bin/
kind create cluster --wait 5m

export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"
kubectl get pods --all-namespaces
