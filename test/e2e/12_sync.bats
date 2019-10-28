#!/usr/bin/env bats

load lib/env
load lib/install
load lib/poll
load lib/defer

git_ssh_cmd=""
git_port_forward_pid=""

function setup() {
  kubectl create namespace "$FLUX_NAMESPACE"
  # Install flux and the git server, allowing external access
  install_git_srv flux-git-deploy git_srv_result
  # shellcheck disable=SC2154
  git_ssh_cmd="${git_srv_result[0]}"
  # shellcheck disable=SC2154
  git_port_forward_pid="${git_srv_result[1]}"
  install_flux_with_fluxctl
}

@test "Basic sync test" {
  # Wait until flux deploys the workloads
  poll_until_true 'workload podinfo' 'kubectl -n demo describe deployment/podinfo'

  # Clone the repo and check the sync tag
  local clone_dir
  clone_dir="$(mktemp -d)"
  defer rm -rf "$clone_dir"
  export GIT_SSH_COMMAND="$git_ssh_cmd"
  git clone -b master ssh://git@localhost/git-server/repos/cluster.git "$clone_dir"
  cd "$clone_dir"
  local sync_tag_hash
  sync_tag_hash=$(git rev-list -n 1 flux)
  head_hash=$(git rev-list -n 1 HEAD)
  [ "$sync_tag_hash" = "$head_hash" ]

  # Add a change, wait for it to happen and check the sync tag again
  sed -i'.bak' 's%stefanprodan/podinfo:2.1.0%stefanprodan/podinfo:3.1.5%' "${clone_dir}/workloads/podinfo-dep.yaml"
  git -c 'user.email=foo@bar.com' -c 'user.name=Foo' commit -am "Bump podinfo"
  head_hash=$(git rev-list -n 1 HEAD)
  git push
  poll_until_equals "podinfo image" "stefanprodan/podinfo:3.1.5" "kubectl get pod -n demo -l app=podinfo -o\"jsonpath={['items'][0]['spec']['containers'][0]['image']}\""
  git pull -f --tags
  sync_tag_hash=$(git rev-list -n 1 flux)
  [ "$sync_tag_hash" = "$head_hash" ]
}

function teardown() {
  kill "$git_port_forward_pid"
  # Removing the namespace also takes care of removing Flux and gitsrv.
  kubectl delete namespace "$FLUX_NAMESPACE"
  # Only remove the demo workloads after Flux, so that they cannot be recreated.
  kubectl delete namespace "$DEMO_NAMESPACE"
}
