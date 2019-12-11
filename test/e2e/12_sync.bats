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

@test "Basic sync test" {
  # Wait until flux deploys the workloads
  poll_until_true 'workload podinfo' 'kubectl -n demo describe deployment/podinfo'

  # Check the sync tag
  local head_hash
  head_hash=$(git rev-list -n 1 HEAD)
  poll_until_equals "sync tag" "$head_hash" 'git pull -f --tags > /dev/null 2>&1; git rev-list -n 1 flux'

  # Add a change, wait for it to happen and check the sync tag again
  sed -i'.bak' 's%stefanprodan/podinfo:.*%stefanprodan/podinfo:3.1.5%' "${clone_dir}/workloads/podinfo-dep.yaml"
  git -c 'user.email=foo@bar.com' -c 'user.name=Foo' commit -am "Bump podinfo"
  head_hash=$(git rev-list -n 1 HEAD)
  git push >&3
  poll_until_equals "podinfo image" "stefanprodan/podinfo:3.1.5" "kubectl get pod -n demo -l app=podinfo -o\"jsonpath={['items'][0]['spec']['containers'][0]['image']}\""
  poll_until_equals "sync tag" "$head_hash" 'git pull -f --tags > /dev/null 2>&1; git rev-list -n 1 flux'

}

@test "Sync fails on duplicate resource" {
  # Wait until flux deploys the workloads
  poll_until_true 'workload podinfo' 'kubectl -n demo describe deployment/podinfo'

  # Check the sync tag
  git pull -f --tags
  local sync_tag_hash
  sync_tag_hash=$(git rev-list -n 1 flux)
  local head_hash
  head_hash=$(git rev-list -n 1 HEAD)
  [ "$head_hash" = "$sync_tag_hash" ]
  podinfo_image=$(kubectl get pod -n demo -l app=podinfo -o"jsonpath={['items'][0]['spec']['containers'][0]['image']}")

  # Bump the image of podinfo, duplicate the resource definition (to cause a sync failure)
  # and make sure the sync doesn't go through
  sed -i'.bak' 's%stefanprodan/podinfo:.*%stefanprodan/podinfo:3.1.5%' "${clone_dir}/workloads/podinfo-dep.yaml"
  cp "${clone_dir}/workloads/podinfo-dep.yaml" "${clone_dir}/workloads/podinfo-dep-2.yaml"
  git add "${clone_dir}/workloads/podinfo-dep-2.yaml"
  git -c 'user.email=foo@bar.com' -c 'user.name=Foo' commit -am "Bump podinfo and duplicate it to cause an error"
  git push
  # Wait until we find the duplicate failure in the logs
  poll_until_true "duplicate resource in Flux logs" "kubectl logs -n $FLUX_NAMESPACE deploy/flux | grep -q \"duplicate definition of 'demo:deployment/podinfo'\""
  # Make sure that the version of podinfo wasn't bumped
  local podinfo_image_now
  podinfo_image_now=$(kubectl get pod -n demo -l app=podinfo -o"jsonpath={['items'][0]['spec']['containers'][0]['image']}")
  [ "$podinfo_image" = "$podinfo_image_now" ]
  # Make sure that the Flux sync tag remains untouched
  git pull -f --tags
  sync_tag_hash=$(git rev-list -n 1 flux)
  [ "$head_hash" = "$sync_tag_hash" ]
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
