#!/usr/bin/env bash

# Echo the various directories that will be used.

set -o errexit

source $(dirname $0)/e2e-paths.env
source $(dirname $0)/machine.env

echo REPO_ROOT=$REPO_ROOT
echo BASE=$BASE
echo GOBIN=$GOBIN
echo OS=$OS
echo ARCH=$ARCH
