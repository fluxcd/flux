#!/usr/bin/env bash

export FLUX_NAMESPACE=flux-e2e
export DEMO_NAMESPACE=demo
FLUX_ROOT_DIR=$(git rev-parse --show-toplevel)
export FLUX_ROOT_DIR
export E2E_DIR="${FLUX_ROOT_DIR}/test/e2e"
export FIXTURES_DIR="${E2E_DIR}/fixtures"
KNOWN_HOSTS=$(cat "${FIXTURES_DIR}/known_hosts")
export KNOWN_HOSTS
GITCONFIG=$(cat "${FIXTURES_DIR}/gitconfig")
export GITCONFIG
