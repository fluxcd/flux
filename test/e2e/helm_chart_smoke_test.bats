#!/usr/bin/env bats

load lib/install

function setup() {
  install_git_srv
  install_tiller
  install_flux_with_helm
}


@test "Helm chart installation smoke test" {

  echo -n '>>> Waiting for gitconfig secret ' >&3
  retries=24
  count=0
  ok=false
  until ${ok}; do
    actual=$(kubectl get secrets -n "${FLUX_NAMESPACE}" gitconfig -ojsonpath={..data.gitconfig} | base64 --decode)
    if [ "${actual}" = "${GITCONFIG}" ]; then
        kubectl get secrets -n "${FLUX_NAMESPACE}" gitconfig
        ok=true
    else
        echo -n '.' >&3
        ok=false
        sleep 5
    fi
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
        echo ' No more retries left' >&3
        kubectl -n "${FLUX_NAMESPACE}" get secrets
        false
    fi
  done
  echo ' done' >&3
  
  echo -n ">>> Waiting for namespace ${DEMO_NAMESPACE} " >&3
  retries=24
  count=1
  ok=false
  until ${ok}; do
    kubectl describe "ns/${DEMO_NAMESPACE}" && ok=true || ok=false
    echo -n '.' >&3
    sleep 5
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
      echo ' No more retries left'
      false
    fi
  done
  echo ' done' >&3

  echo -n '>>> Waiting for workload podinfo ' >&3
  retries=24
  count=0
  ok=false
  until ${ok}; do
    kubectl -n "${DEMO_NAMESPACE}" describe deployment/podinfo && ok=true || ok=false
    echo -n '.' >&3
    sleep 5
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
      echo ' No more retries left' >&3
      false
    fi
  done
  echo ' done' >&3

  echo -n '>>> Waiting for mongodb HelmRelease ' >&3
  retries=24
  count=0
  ok=false
  until ${ok}; do
    kubectl -n $DEMO_NAMESPACE describe helmrelease/mongodb && ok=true || ok=false
    echo -n '.' >&3
    sleep 5
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
      echo ' No more retries left' >&3
      false
    fi
  done
  echo ' done' >&3
  
  echo '>>> List pods' >&3
  kubectl -n "${DEMO_NAMESPACE}" get pods

  echo '>>> Check workload' >&3
  kubectl -n "${DEMO_NAMESPACE}" rollout status deployment/podinfo

}

function teardown() {
  # For debugging purposes (in case the test fails)
  echo '>>> Flux logs'
  kubectl -n "${FLUX_NAMESPACE}" logs deployment/flux
  echo '>>> Helm Operator logs'
  kubectl -n "${FLUX_NAMESPACE}" logs deployment/flux-helm-operator
  
  uninstall_flux_with_helm
  uninstall_tiller
  uninstall_git_srv
  kubectl delete namespace demo
}
