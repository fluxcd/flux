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

  # Check that all the resources in the demo namespace are created by Flux
  poll_until_true "namespace ${DEMO_NAMESPACE}" "kubectl describe ns/${DEMO_NAMESPACE}"
  poll_until_true 'workload podinfo' "kubectl -n "${DEMO_NAMESPACE}" describe deployment/podinfo"
  poll_until_true 'mongodb HelmRelease' "kubectl -n ${DEMO_NAMESPACE} describe helmrelease/mongodb"

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
