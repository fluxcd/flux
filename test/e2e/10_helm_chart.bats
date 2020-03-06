#!/usr/bin/env bats

function setup() {
  load lib/env
  load lib/install
  load lib/poll

  kubectl create namespace "$FLUX_NAMESPACE"
  install_git_srv
  install_tiller
  install_flux_with_helm
}

@test "Helm chart installation smoke test" {
  # The gitconfig secret must exist and have the right value
  poll_until_equals "gitconfig secret" "${GITCONFIG}" "kubectl get secrets -n ${FLUX_NAMESPACE} gitconfig -ojsonpath={..data.gitconfig} | base64 --decode"

  # Test that the resources from https://github.com/fluxcd/flux-get-started are deployed
  poll_until_true 'namespace demo' 'kubectl describe ns/demo'
  poll_until_true 'workload podinfo' 'kubectl -n demo describe deployment/podinfo'
}

function teardown() {
  # Removing Flux also takes care of the global resources it installs.
  uninstall_flux_with_helm
  uninstall_tiller
  # Removing the namespace also takes care of removing gitsrv.
  kubectl delete namespace "$FLUX_NAMESPACE"
  # Only remove the demo workloads after Flux, so that they cannot be recreated.
  kubectl delete namespace "$DEMO_NAMESPACE"
}
