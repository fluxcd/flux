#!/usr/bin/env bash

# Build the images, assuming the checkout is in the expected Go
# directory layout. If running this locally, you can just use
#
#    dep ensure; make all
#
# since you'll already be set up to build images. Make sure you are
# pointed at the docker that `kind` is using.

set -o errexit

source $(dirname $0)/e2e-paths.env

echo ">>> Installing go dep to $GOBIN"
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
dep ensure -vendor-only

echo ">>> Building docker images"
make all
