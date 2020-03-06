#!/usr/bin/env bats

function setup() {
  load lib/env
  load lib/install
  load lib/poll
  load lib/defer

  kubectl create namespace "$FLUX_NAMESPACE"
  # Install flux and the git server, allowing external access
  install_git_srv git_srv_result "22_manifest_generation/gitsrv"
  # shellcheck disable=SC2154
  export GIT_SSH_COMMAND="${git_srv_result[0]}"
  # Teardown the created port-forward to gitsrv.
  defer kill "${git_srv_result[1]}"
  install_flux_with_fluxctl "22_manifest_generation/flux"
}

@test "Basic sync and editing" {
  # Wait until flux deploys the workloads
  poll_until_true 'workload podinfo' 'kubectl -n demo describe deployment/podinfo'

  # Make sure that the production patch is applied (the podinfo HorizontalPodAutoscaler should have
  # a minReplicas value of 2)
  poll_until_equals 'podinfo hpa minReplicas of 2' '2' "kubectl get hpa podinfo --namespace demo -o\"jsonpath={['spec']['minReplicas']}\""

  # Make sure the 'patchUpdated' mechanism works when changing annotations through fluxctl
  fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" automate -n demo --workload deployment/podinfo >&3

  poll_until_true 'podinfo to be automated' "fluxctl --k8s-fwd-ns \"${FLUX_NAMESPACE}\" list-workloads -n demo | grep podinfod | grep automated"

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
