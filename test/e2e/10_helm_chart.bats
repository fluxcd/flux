#!/usr/bin/env bats

load lib/install
load lib/poll

function setup() {
  install_git_srv
  install_tiller
  install_flux_with_helm
}

@test "Helm chart installation smoke test" {
  # The gitconfig secret must exist and have the right value
  poll_until_equals "gitconfig secret" "${GITCONFIG}" "kubectl get secrets -n "${FLUX_NAMESPACE}" gitconfig -ojsonpath={..data.gitconfig} | base64 --decode"

  # Test that the resources from https://github.com/fluxcd/flux-get-started are deployed
  poll_until_true 'namespace demo' 'kubectl describe ns/demo'
  poll_until_true 'workload podinfo' 'kubectl -n demo describe deployment/podinfo'
  poll_until_true 'mongodb HelmRelease' 'kubectl -n demo describe helmrelease/mongodb'
}

function teardown() {
  # For debugging purposes (in case the test fails)
  echo '>>> Flux logs'
  kubectl -n "${FLUX_NAMESPACE}" logs deployment/flux
  echo '>>> List pods'
  kubectl -n "${DEMO_NAMESPACE}" get pods
  echo '>>> Check workload'
  kubectl -n "${DEMO_NAMESPACE}" rollout status deployment/podinfo

  uninstall_flux_with_helm
  uninstall_tiller
  uninstall_git_srv
  kubectl delete namespace demo
}
