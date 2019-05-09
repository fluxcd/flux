#!/usr/bin/env bash

set -o errexit

GO_VERSION=1.12.5

echo ">>> Installing go ${GO_VERSION}"
curl -O https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz
tar -xf go${GO_VERSION}.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo mv go /usr/local

export GOPATH=$HOME/go
export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

mkdir -p $HOME/go/bin
mkdir -p $HOME/go/src

go version
