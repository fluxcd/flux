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
# Hairy way to clone and build version 0.2.1 of Kind since it doesn't support Go Modules:
mkdir -p $GOPATH/src/sigs.k8s.io
git clone https://github.com/kubernetes-sigs/kind.git $GOPATH/src/sigs.k8s.io/kind
git -C $GOPATH/src/sigs.k8s.io/kind checkout tags/0.2.1
go install sigs.k8s.io/kind

echo ">>> Installing kind"
sudo cp $GOPATH/bin/kind /usr/local/bin/
kind create cluster --wait 5m

export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"
kubectl get pods --all-namespaces
