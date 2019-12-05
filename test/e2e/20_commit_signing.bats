#!/usr/bin/env bats

function setup() {
  load lib/defer
  load lib/env
  load lib/gpg
  load lib/install
  load lib/poll

  kubectl create namespace "${FLUX_NAMESPACE}" &> /dev/null

  # Install the git server, allowing external access
  install_git_srv git_srv_result
  # shellcheck disable=SC2154
  export GIT_SSH_COMMAND="${git_srv_result[0]}"
  # Teardown the created port-forward to gitsrv.
  defer kill "${git_srv_result[1]}"

  # Create a temporary GNUPGHOME
  local tmp_gnupghome
  tmp_gnupghome=$(mktemp -d)
  export GNUPGHOME="$tmp_gnupghome"
  defer rm -rf "'$tmp_gnupghome'"

  # Install Flux, with a new GPG key and signing enabled
  local gpg_key
  gpg_key=$(create_gpg_key)
  create_secret_from_gpg_key "$gpg_key"
  local -A template_values
  # shellcheck disable=SC2034
  template_values['FLUX_GPG_KEY_ID']="$gpg_key"
  # shellcheck disable=SC2034
  template_values['FLUX_GIT_VERIFY_SIGNATURES']="false"
  install_flux_with_fluxctl '20_gpg/flux' 'template_values'
}

@test "Git sync tag is signed" {
  # Test that a resource from https://github.com/fluxcd/flux-get-started is deployed
  # This means the Flux instance _should_ have pushed a signed high-watermark tag
  poll_until_true 'namespace demo' 'kubectl describe ns/demo'

  # Clone the repo
  local clone_dir
  clone_dir="$(mktemp -d)"
  defer rm -rf "'$clone_dir'"
  git clone -b master ssh://git@localhost/git-server/repos/cluster.git "$clone_dir"
  cd "$clone_dir"

  # Test that the tag has been signed, this errors if this isn't the case
  git pull -f --tags
  git verify-tag --raw flux >&3
}

@test "Git commits are signed" {
  # Ensure the resource we are going to lock is deployed
  poll_until_true 'workload podinfo' 'kubectl -n demo describe deployment/podinfo'

  # Let Flux push a commit
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" lock --workload demo:deployment/podinfo >&3

  # Clone the repo
  local clone_dir
  clone_dir="$(mktemp -d)"
  defer rm -rf "'$clone_dir'"
  git clone -b master ssh://git@localhost/git-server/repos/cluster.git "$clone_dir"
  cd "$clone_dir"

  # Test that the commit has been signed, this errors if this isn't the case
  git verify-commit --raw HEAD >&3
}

function teardown() {
  run_deferred
  # Kill the agent and remove temporary GNUPGHOME
  gpgconf --kill gpg-agent
  # Uninstall Flux and the global resources it installs.
  uninstall_flux_with_fluxctl
  # Removing the namespace also takes care of removing Flux and gitsrv.
  kubectl delete namespace "$FLUX_NAMESPACE"
  # Only remove the demo workloads after Flux, so that they cannot be recreated.
  kubectl delete namespace "$DEMO_NAMESPACE"
}
