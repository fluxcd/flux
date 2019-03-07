#!/usr/bin/env bash

set -o errexit

curl -O https://storage.googleapis.com/golang/go1.11.4.linux-amd64.tar.gz
tar -xvf go1.11.4.linux-amd64.tar.gz
sudo mv go /usr/local

export GOPATH=$HOME/go
export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

go version

curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
dep ensure -vendor-only

make all
