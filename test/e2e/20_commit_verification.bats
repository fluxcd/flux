#!/usr/bin/env bats

function setup() {
  load lib/env
  load lib/gpg
  load lib/install
  load lib/poll
  load lib/defer

  kubectl create namespace "${FLUX_NAMESPACE}"

  # Create a temporary GNUPGHOME
  tmp_gnupghome=$(mktemp -d)
  defer rm -rf "'$tmp_gnupghome'"
  export GNUPGHOME="$tmp_gnupghome"
}

@test "Commits are verified" {
  # Create a new GPG key and secret
  gpg_key=$(create_gpg_key)
  create_secret_from_gpg_key "$gpg_key"

  # Install the git server with signed init commit,
  # allowing external access
  install_git_srv git_srv_result 20_gpg/gitsrv

  # Install Flux with the GPG key, and commit verification enabled
  local -A template_values
  # shellcheck disable=SC2034
  template_values['FLUX_GPG_KEY_ID']="$gpg_key"
  # shellcheck disable=SC2034
  template_values['FLUX_GIT_VERIFY_SIGNATURES']="true"
  install_flux_with_fluxctl '20_gpg/flux' 'template_values'

  # shellcheck disable=SC2154
  git_ssh_cmd="${git_srv_result[0]}"
  export GIT_SSH_COMMAND="$git_ssh_cmd"

  # shellcheck disable=SC2030
  defer kill "${git_srv_result[1]}"

  # Test that the resources from https://github.com/fluxcd/flux-get-started are deployed
  poll_until_true 'namespace demo' 'kubectl describe ns/demo'

  # Clone the repo
  # shellcheck disable=SC2030
  clone_dir="$(mktemp -d)"
  defer rm -rf "'$clone_dir'"
  git clone -b master ssh://git@localhost/git-server/repos/cluster.git "$clone_dir"
  cd "$clone_dir"

  local sync_tag="flux"
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
  install_git_srv

  # Install Flux with the GPG key, and commit verification enabled
  local -A template_values
  # shellcheck disable=SC2034
  template_values['FLUX_GPG_KEY_ID']="$gpg_key"
  # shellcheck disable=SC2034
  template_values['FLUX_GIT_VERIFY_SIGNATURES']="true"
  install_flux_with_fluxctl '20_gpg/flux' 'template_values'

  # Wait for Flux to report that it sees an invalid commit
  poll_until_true 'invalid GPG signature log' "kubectl logs -n ${FLUX_NAMESPACE} deploy/flux | grep -q -e 'found invalid GPG signature for commit'"

  # Attempt to lock a resource, and confirm it returns an error.
  run fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" lock --workload demo:deployment/podinfo
  [ "$status" -eq 1 ]
  [[ "$output" == *"Error: HEAD revision is unsigned"* ]]
}

function teardown() {
  run_deferred
  # Kill the agent
  gpgconf --kill gpg-agent
  # Although the namespace delete below takes care of removing most Flux
  # elements, the global resources will not be removed without this.
  uninstall_flux_with_fluxctl
  # Removing the namespace also takes care of removing Flux and gitsrv.
  kubectl delete namespace "$FLUX_NAMESPACE"
  # (Maybe) remove the demo namespace
  kubectl delete namespace "$DEMO_NAMESPACE" &> /dev/null || true
}
