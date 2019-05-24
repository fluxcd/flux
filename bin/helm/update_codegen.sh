#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/../..
CODEGEN_PKG=${CODEGEN_PKG:-$(echo `go env GOPATH`'/pkg/mod/k8s.io/code-generator@v0.0.0-20190511023357-639c964206c2')}

go mod download # make sure the code-generator is downloaded
bash ${CODEGEN_PKG}/generate-groups.sh all github.com/weaveworks/flux/integrations/client \
              github.com/weaveworks/flux/integrations/apis \
              "flux.weave.works:v1beta1 helm.integrations.flux.weave.works:v1alpha2" \
  --go-header-file "${SCRIPT_ROOT}/bin/helm/custom-boilerplate.go.txt"

