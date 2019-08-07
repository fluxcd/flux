#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# This corresponds to the tag kubernetes-1.14.4, and to the pinned version in go.mod
CODEGEN_VERSION="code-generator@v0.0.0-20190311093542-50b561225d70"
SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/../..

# Chosen to line up with the ./build directory used in the Makefile
CODEGEN_ROOT=./build/codegen
# make sure this exists ...
mkdir -p ./build
# ... but not this
rm -rf ${CODEGEN_ROOT}

echo Using codegen in ${CODEGEN_ROOT}

export GO111MODULE=on
# make sure the codegen module has been fetched
go mod download
cp -R $(echo `go env GOPATH`)'/pkg/mod/k8s.io/'${CODEGEN_VERSION} ${CODEGEN_ROOT}
chmod -R u+w ${CODEGEN_ROOT}

bash "${CODEGEN_ROOT}/generate-groups.sh" all github.com/weaveworks/flux/integrations/client \
              github.com/weaveworks/flux/integrations/apis \
              "flux.weave.works:v1beta1 helm.integrations.flux.weave.works:v1alpha2" \
  --go-header-file "${SCRIPT_ROOT}/bin/helm/custom-boilerplate.go.txt"
