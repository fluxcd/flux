#!/usr/bin/env bash

set -o errexit

# This script runs the bats tests, first ensuring there is a kubernetes cluster available,
# with a flux namespace and a git secret ready to use

# Directory paths we need to be aware of
FLUX_ROOT_DIR="$(git rev-parse --show-toplevel)"
E2E_DIR="${FLUX_ROOT_DIR}/test/e2e"
CACHE_DIR="${FLUX_ROOT_DIR}/cache/$CURRENT_OS_ARCH"

KIND_VERSION=v0.7.0
KUBE_VERSION=v1.14.10
KIND_CACHE_PATH="${CACHE_DIR}/kind-$KIND_VERSION"
KIND_CLUSTER_PREFIX=flux-e2e
BATS_EXTRA_ARGS=""

# shellcheck disable=SC1090
source "${E2E_DIR}/lib/defer.bash"
trap run_deferred EXIT

function install_kind() {
  if [ ! -f "${KIND_CACHE_PATH}" ]; then
    echo '>>> Downloading Kind'
    mkdir -p "${CACHE_DIR}"
    curl -sL "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-${CURRENT_OS_ARCH}" -o "${KIND_CACHE_PATH}"
  fi
  cp "${KIND_CACHE_PATH}" "${FLUX_ROOT_DIR}/test/bin/kind"
  chmod +x "${FLUX_ROOT_DIR}/test/bin/kind"
}

# Create multiple Kind clusters and run jobs in parallel?
# Let users specify how many, e.g. with E2E_KIND_CLUSTER_NUM=3 make e2e
E2E_KIND_CLUSTER_NUM=${E2E_KIND_CLUSTER_NUM:-1}

# Check if there is a kubernetes cluster running, otherwise use Kind
if ! kubectl version > /dev/null 2>&1; then
  install_kind

  # We require GNU Parallel, but some systems come with Tollef's parallel (moreutils)
  if ! parallel -h | grep -q "GNU Parallel"; then
    echo "GNU Parallel is not available on your system"
    exit 1
  fi

  echo '>>> Creating Kind Kubernetes cluster(s)'
  KIND_CONFIG_PREFIX="${HOME}/.kube/kind-config-${KIND_CLUSTER_PREFIX}"
  for I in $(seq 1 "${E2E_KIND_CLUSTER_NUM}"); do
    defer kind --name "${KIND_CLUSTER_PREFIX}-${I}" delete cluster > /dev/null 2>&1 || true
    defer rm -rf "${KIND_CONFIG_PREFIX}-${I}"
    # Wire tests with the right cluster based on their BATS_JOB_SLOT env variable
    eval export "KUBECONFIG_SLOT_${I}=${KIND_CONFIG_PREFIX}-${I}"
  done
  seq 1 "${E2E_KIND_CLUSTER_NUM}" | time parallel -- env KUBECONFIG="${KIND_CONFIG_PREFIX}-{}" kind -v 1 create cluster --name "${KIND_CLUSTER_PREFIX}-{}" --wait 5m --image kindest/node:${KUBE_VERSION}

  echo '>>> Loading images into the Kind cluster(s)'
  seq 1 "${E2E_KIND_CLUSTER_NUM}" | time parallel -- kind --name "${KIND_CLUSTER_PREFIX}-{}" load docker-image 'docker.io/fluxcd/flux:latest'
  if [ "${E2E_KIND_CLUSTER_NUM}" -gt 1 ]; then
    BATS_EXTRA_ARGS="--jobs ${E2E_KIND_CLUSTER_NUM}"
  fi
fi

echo '>>> Running the tests'
# Run all tests by default but let users specify which ones to run, e.g. with E2E_TESTS='11_*' make e2e
E2E_TESTS=${E2E_TESTS:-.}
(
  cd "${E2E_DIR}"
  # shellcheck disable=SC2086
  "${E2E_DIR}/bats/bin/bats" -t ${BATS_EXTRA_ARGS} ${E2E_TESTS}
)
