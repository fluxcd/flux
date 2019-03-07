#!/usr/bin/env bash

set -o errexit

export GOPATH=$HOME/go
export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
dep ensure -vendor-only

make all

