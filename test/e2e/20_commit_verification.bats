#!/usr/bin/env bats

load lib/env
load lib/gpg
load lib/install
load lib/poll

tmp_gnupghome=""
git_port_forward_pid=""

function setup() {
  kubectl create namespace "${FLUX_NAMESPACE}"

  # Create a temporary GNUPGHOME
  tmp_gnupghome=$(mktemp -d)
  export GNUPGHOME="$tmp_gnupghome"
}

@test "Commits are verified" {
  # Create a new GPG key and secret
  gpg_key=$(create_gpg_key)
  create_secret_from_gpg_key "$gpg_key"

  # Install the git server with signed init commit,
  # allowing external access
  install_git_srv flux-git-deploy git_srv_result true

  # Install Flux with the GPG key, and commit verification enabled
  install_flux_gpg "$gpg_key" true

  # shellcheck disable=SC2154
  git_ssh_cmd="${git_srv_result[0]}"
  export GIT_SSH_COMMAND="$git_ssh_cmd"

  # shellcheck disable=SC2030
  git_port_forward_pid="${git_srv_result[1]}"
  defer kill "$git_port_forward_pid"

  # Test that the resources from https://github.com/fluxcd/flux-get-started are deployed
  poll_until_true 'namespace demo' 'kubectl describe ns/demo'
  defer kubectl delete namespace "$DEMO_NAMESPACE"

  # Clone the repo
  local clone_dir
  clone_dir="$(mktemp -d)"
  defer rm -rf "$clone_dir"
  git clone -b master ssh://git@localhost/git-server/repos/cluster.git "$clone_dir"
  cd "$clone_dir"

  local sync_tag="flux-sync"
  local org_head_hash
  org_head_hash=$(git rev-list -n 1 HEAD)
  sync_tag_hash=$(git rev-list -n 1 "$sync_tag")

  [ "$sync_tag_hash" = "$org_head_hash" ]
  run git verify-commit "$sync_tag_hash"
  [ "$status" -eq 0 ]

  # Add an unsigned change
  sed -i'.bak' 's%stefanprodan/podinfo:.*%stefanprodan/podinfo:3.1.5%' "${clone_dir}/workloads/podinfo-dep.yaml"
  git -c 'user.email=foo@bar.com' -c 'user.name=Foo' commit -am "Bump podinfo"
  git push

  # Delete tag
  git push --delete origin "$sync_tag"

  # Sync should warn, and put the tag back at the latest verified commit
  run fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" sync
  [ "$status" -eq 0 ]
  [[ "$output" == *"Warning: The branch HEAD in the git repo is not verified"* ]]

  git pull -f --tags
  sync_tag_hash=$(git rev-list -n 1 "$sync_tag")
  [ "$sync_tag_hash" = "$org_head_hash" ]
}

@test "Does not commit on top of invalid commit" {
  # Create a new GPG key and secret
  gpg_key=$(create_gpg_key)
  create_secret_from_gpg_key "$gpg_key"

  # Install the git server with _unsigned_ init commit
  install_git_srv flux-git-deploy "" false

  # Install Flux with the GPG key, and commit verification enabled
  install_flux_gpg "$gpg_key" true

  # Wait for Flux to report that it sees an invalid commit
  poll_until_true 'invalid GPG signature log' "kubectl logs -n ${FLUX_NAMESPACE} deploy/flux-gpg | grep -e 'found invalid GPG signature for commit'"

  # Attempt to lock a resource, and confirm it returns an error.
  run fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" lock --workload demo:deployment/podinfo
  [ "$status" -eq 1 ]
  [[ "$output" == *"Error: HEAD revision is unsigned"* ]]
}

function teardown() {
  # (Maybe) teardown the created port-forward to gitsrv.
  # shellcheck disable=SC2031
  kill "$git_port_forward_pid" || true
  # Kill the agent and remove temporary GNUPGHOME
  gpgconf --kill gpg-agent
  rm -rf "$tmp_gnupghome"
  # Although the namespace delete below takes care of removing most Flux
  # elements, the global resources will not be removed without this.
  uninstall_flux_gpg
  # Removing the namespace also takes care of removing Flux and gitsrv.
  kubectl delete namespace "$FLUX_NAMESPACE"
}
