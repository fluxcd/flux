#!/usr/bin/env bash

set -o errexit

GO_VERSION=1.11.4

echo ">>> Installing go ${GO_VERSION}"
curl -O https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz
tar -xf go1.11.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo mv go /usr/local

export GOPATH=$HOME/go
export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

mkdir -p $HOME/go/bin
mkdir -p $HOME/go/src

go version
