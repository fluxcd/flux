#!/usr/bin/env bash

set -o errexit

export GOPATH=$HOME/go
export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

echo ">>> Installing go dep"
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
dep ensure -vendor-only

echo ">>> Building docker images"
make all
