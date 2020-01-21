#!/usr/bin/env bats

function setup() {
  load lib/env
  load lib/install
  load lib/poll
  load lib/defer
  load lib/registry

  kubectl create namespace "$FLUX_NAMESPACE"

  # Install the git server, allowing external access
  install_git_srv git_srv_result
  # shellcheck disable=SC2154
  export GIT_SSH_COMMAND="${git_srv_result[0]}"
  # Teardown the created port-forward to gitsrv.
  defer kill "${git_srv_result[1]}"

  # Install a local registry, with some empty images to be used later in the test
  install_registry registry_result
  # shellcheck disable=SC2154
  REGISTRY_PORT="${registry_result[0]}"
  # Teardown the created port-forward to the registry.
  defer kill "${registry_result[1]}"
  echo "REGISTRY_PORT=$REGISTRY_PORT" >&3
  # create empty images for the test
  push_empty_image "localhost:$REGISTRY_PORT" 'bitnami/ghost:3.0.2-debian-9-r3' '2020-01-20T13:53:05.47178071Z'
  push_empty_image "localhost:$REGISTRY_PORT" 'bitnami/ghost:3.1.1-debian-9-r0' '2020-02-20T13:53:05.47178071Z'
  push_empty_image "localhost:$REGISTRY_PORT" 'stefanprodan/podinfo:3.1.0' '2020-03-20T13:53:05.47178071Z'
  push_empty_image "localhost:$REGISTRY_PORT" 'stefanprodan/podinfo:3.0.5' '2020-04-20T13:53:05.47178071Z'
  REGISTRY_SERVICE_IP=$(kubectl -n "$FLUX_NAMESPACE" get service registry -o 'jsonpath={.spec.clusterIP}')

  # Finally, install Flux
  local -A template_values
  # shellcheck disable=SC2034
  template_values['REGISTRY_SERVICE_IP']="$REGISTRY_SERVICE_IP"
  install_flux_with_fluxctl '14_release_image' 'template_values'
}

@test "Image releases" {
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

  # Wait for the registry scanner to fo its magic on stefanprodan/podinfo and bitnami/ghost
  poll_until_true "stefanprodan/podinfo to be scanned" "kubectl logs -n $FLUX_NAMESPACE deploy/flux | grep -q \"component=warmer updated=stefanprodan/podinfo\"" 5 50
  poll_until_true "bitnami/ghost to be scanned" "kubectl logs -n $FLUX_NAMESPACE deploy/flux | grep -q \"component=warmer updated=bitnami/ghost\"" 5 50

  # Manually release podinfo to version 3.0.5
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" deautomate --workload=demo:deployment/podinfo
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" release --force --workload=demo:deployment/podinfo --update-image=stefanprodan/podinfo:3.0.5
  poll_until_true "deployment/podinfo version 3.0.5 to be released" 'git pull > /dev/null 2>&1; grep -q stefanprodan/podinfo:3.0.5 workloads/podinfo-dep.yaml'

  # Manually release ghost to version 3.0.2-debian-9-r3
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" release --force --workload=demo:helmrelease/ghost --update-image=bitnami/ghost:3.0.2-debian-9-r3
  poll_until_true "helmrelease/ghost version 3.0.2-debian-9-r3 to be released" 'git pull > /dev/null 2>&1; grep -q 3.0.2-debian-9-r3 releases/ghost.yaml'

  # Automate the resources and make sure that podinfo and ghost are updated according to their annotations
  # (semver:~3.1 for podinfo and glob:3.1.1-debian-9-* for ghost)
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" automate --workload=demo:deployment/podinfo
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" automate --workload=demo:helmrelease/ghost
  poll_until_true "deployment/podinfo semver:~3.1 to be released" 'git pull > /dev/null 2>&1; grep -q stefanprodan/podinfo:3.1. workloads/podinfo-dep.yaml'
  poll_until_true "helmrelease/ghost glob:3.1.1-debian-9-* to be released" 'git pull > /dev/null 2>&1; grep -q 3.1.1-debian-9-r0 releases/ghost.yaml'
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
