#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/../..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ${GOPATH}/src/k8s.io/code-generator)}

${CODEGEN_PKG}/generate-groups.sh all github.com/weaveworks/flux/integrations/client \
              github.com/weaveworks/flux/integrations/apis \
              "flux.weave.works:v1beta1 helm.integrations.flux.weave.works:v1alpha2" \
  --go-header-file ${SCRIPT_ROOT}/bin/helm/custom-boilerplate.go.txt

