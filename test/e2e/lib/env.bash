#!/usr/bin/env bash

function setup_env() {
    local flux_namespace=${1:-flux-e2e}
    export FLUX_NAMESPACE=$flux_namespace

    local demo_namespace=${2:-demo}
    export DEMO_NAMESPACE=$demo_namespace

    local flux_root_dir
    flux_root_dir=$(git rev-parse --show-toplevel)
    export FLUX_ROOT_DIR=$flux_root_dir
    export E2E_DIR="${FLUX_ROOT_DIR}/test/e2e"
    export FIXTURES_DIR="${E2E_DIR}/fixtures"

    local known_hosts
    known_hosts=$(cat "${FIXTURES_DIR}/known_hosts")
    export KNOWN_HOSTS=$known_hosts

    local gitconfig
    gitconfig=$(cat "${FIXTURES_DIR}/gitconfig")
    export GITCONFIG=$gitconfig
}
