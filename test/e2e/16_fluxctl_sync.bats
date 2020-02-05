#!/usr/bin/env bats

clone_dir=""

function setup() {
  load lib/env
  load lib/install
  load lib/poll
  load lib/defer

  kubectl create namespace "$FLUX_NAMESPACE"
  # Install flux and the git server, allowing external access
  install_git_srv git_srv_result
  # shellcheck disable=SC2154
  export GIT_SSH_COMMAND="${git_srv_result[0]}"
  # Teardown the created port-forward to gitsrv and restore Git settings.
  defer kill "${git_srv_result[1]}"

  install_flux_with_fluxctl '15_fluxctl_sync'

  # Clone the repo
  clone_dir="$(mktemp -d)"
  defer rm -rf "'$clone_dir'"
  git clone -b master ssh://git@localhost/git-server/repos/cluster.git "$clone_dir"
  # shellcheck disable=SC2164
  cd "$clone_dir"
}

@test "fluxctl sync" {

  # Sync
  poll_until_true 'fluxctl sync succeeds' "fluxctl --k8s-fwd-ns ${FLUX_NAMESPACE} sync"

  # Wait until flux deploys the workloads
  poll_until_true 'workload podinfo' 'kubectl -n demo describe deployment/podinfo'

  # Check the sync tag
  local head_hash
  head_hash=$(git rev-list -n 1 HEAD)
  poll_until_equals "sync tag" "$head_hash" 'git pull -f --tags > /dev/null 2>&1; git rev-list -n 1 flux'

}

function teardown() {
  run_deferred
  # Although the namespace delete below takes care of removing most Flux
  # elements, the global resources will not be removed without this.
  uninstall_flux_with_fluxctl
  # Removing the namespace also takes care of removing gitsrv.
  kubectl delete namespace "$FLUX_NAMESPACE"
  # Only remove the demo workloads after Flux, so that they cannot be recreated.
  kubectl delete namespace "$DEMO_NAMESPACE"
}
