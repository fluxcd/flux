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

  install_flux_with_fluxctl

  # Clone the repo
  clone_dir="$(mktemp -d)"
  defer rm -rf "'$clone_dir'"
  git clone -b master ssh://git@localhost/git-server/repos/cluster.git "$clone_dir"
  # shellcheck disable=SC2164
  cd "$clone_dir"
}

# TODO: make annotation checks more precise with a yaml-aware tool as opposed to simply grepping for precense
#       anywhere in the files
@test "fluxctl policy/(de)automate/(un)lock" {

  # Check that podinfo is starting up in the state assumed by the test
  grep -q 'fluxcd.io/automated: "true"' workloads/podinfo-dep.yaml # automated
  ! grep -q 'fluxcd.io/locked' workloads/podinfo-dep.yaml          # unlocked
  grep -q 'fluxcd.io/tag.init: regex:^3.10.*' workloads/podinfo-dep.yaml
  grep -q 'fluxcd.io/tag.podinfod: semver:~3.1' workloads/podinfo-dep.yaml

  ###########
  ## Automate
  ###########

  # de-automate (polling since Flux may not be ready yet)
  poll_until_true 'fluxctl deautomate' "fluxctl --k8s-fwd-ns ${FLUX_NAMESPACE} deautomate --workload=demo:deployment/podinfo"
  git pull
  ! grep -q 'fluxcd.io/automated' workloads/podinfo-dep.yaml

  # re-automate
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" automate --workload=demo:deployment/podinfo
  git pull
  grep -q "fluxcd.io/automated: 'true'" workloads/podinfo-dep.yaml

  # de-automate again, with the policy command
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" policy --deautomate --workload=demo:deployment/podinfo
  git pull
  ! grep -q 'fluxcd.io/automated' workloads/podinfo-dep.yaml

  # re-automate, with the policy command
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" policy --automate --workload=demo:deployment/podinfo
  git pull
  grep -q "fluxcd.io/automated: 'true'" workloads/podinfo-dep.yaml

  #######
  ## Lock
  #######

  # lock
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" lock --workload=demo:deployment/podinfo
  git pull
  grep -q "fluxcd.io/locked: 'true'" workloads/podinfo-dep.yaml

  # unlock
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" unlock --workload=demo:deployment/podinfo
  git pull
  ! grep -q 'fluxcd.io/locked' workloads/podinfo-dep.yaml

  # re-lock, with the policy command
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" policy --lock --workload=demo:deployment/podinfo
  git pull
  grep -q "fluxcd.io/locked: 'true'" workloads/podinfo-dep.yaml

  # unlock again, with the policy command
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" policy --unlock --workload=demo:deployment/podinfo
  git pull
  ! grep -q 'fluxcd.io/locked' workloads/podinfo-dep.yaml

  ##############
  ## Policy tags
  ##############

  # Update podinfo tag
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" policy --workload=demo:deployment/podinfo --tag='podinfod=3.5.*'
  git pull
  grep -q "fluxcd.io/tag.podinfod: glob:3.5.*" workloads/podinfo-dep.yaml

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
