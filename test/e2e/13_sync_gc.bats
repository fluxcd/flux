#!/usr/bin/env bats

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
  # Teardown the created port-forward to gitsrv.
  defer kill "${git_srv_result[1]}"
  install_flux_with_fluxctl "13_sync_gc"
}

@test "Sync with garbage collection test" {
  # Wait until flux deploys the workloads, which indicates it has at least started a sync
  poll_until_true 'workload podinfo' 'kubectl -n demo describe deployment/podinfo'

  # make sure we have _finished_ a sync run
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" sync

  # Clone the repo and check the sync tag
  local clone_dir
  clone_dir="$(mktemp -d)"
  defer rm -rf "'$clone_dir'"
  git clone -b master ssh://git@localhost/git-server/repos/cluster.git "$clone_dir"
  cd "$clone_dir"
  head_hash=$(git rev-list -n 1 HEAD)
  poll_until_equals "sync tag" "$head_hash" 'git pull -f --tags > /dev/null 2>&1; git rev-list -n 1 flux'

  # Remove a manifest and commit that
  git rm workloads/podinfo-dep.yaml
  git -c 'user.email=foo@bar.com' -c 'user.name=Foo' commit -m "Remove podinfo deployment"
  head_hash=$(git rev-list -n 1 HEAD)
  git push >&3

  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" sync

  poll_until_equals "podinfo deployment removed" "[]" "kubectl get deploy -n demo -o\"jsonpath={['items']}\""
  poll_until_equals "sync tag" "$head_hash" 'git pull -f --tags > /dev/null 2>&1; git rev-list -n 1 flux'
}

function teardown() {
  run_deferred
  # Although the namespace delete below takes care of removing most Flux
  # elements, the global resources will not be removed without this.
  uninstall_flux_with_fluxctl
  # Removing the namespace also takes care of removing Flux and gitsrv.
  kubectl delete namespace "$FLUX_NAMESPACE"
  # Only remove the demo workloads after Flux, so that they cannot be recreated.
  kubectl delete namespace "$DEMO_NAMESPACE"
}
