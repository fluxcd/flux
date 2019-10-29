#!/usr/bin/env bats

load lib/env
load lib/install
load lib/poll

function setup() {
  setup_env
  kubectl create namespace "$FLUX_NAMESPACE"
  generate_ssh_secret
  install_git_srv
  install_flux_with_fluxctl
}

@test "'fluxctl install' smoke test" {
  # Test that the resources from https://github.com/fluxcd/flux-get-started are deployed
  poll_until_true 'namespace demo' 'kubectl describe ns/demo'
  poll_until_true 'workload podinfo' 'kubectl -n demo describe deployment/podinfo'
}

function teardown() {
  # For debugging purposes (in case the test fails)
  echo '>>> Flux logs'
  kubectl -n "${FLUX_NAMESPACE}" logs deployment/flux
  echo '>>> List pods'
  kubectl -n "${DEMO_NAMESPACE}" get pods
  echo '>>> Check workload'
  kubectl -n "${DEMO_NAMESPACE}" rollout status deployment/podinfo

  uninstall_flux_with_fluxctl
  uninstall_git_srv
  kubectl delete namespace "$DEMO_NAMESPACE"
  # This also takes care of removing the generated secret
  kubectl delete namespace "$FLUX_NAMESPACE"
}
