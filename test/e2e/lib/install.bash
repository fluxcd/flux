#!/usr/bin/env bash

# shellcheck disable=SC1090
source "${E2E_DIR}/lib/defer.bash"
# shellcheck disable=SC1090
source "${E2E_DIR}/lib/template.bash"
# shellcheck disable=SC1090
source "${E2E_DIR}/lib/poll.bash"

function install_tiller() {
  if ! helm version > /dev/null 2>&1; then # only if helm isn't already installed
    kubectl --namespace kube-system create sa tiller
    kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
    helm init --service-account tiller --upgrade --wait
  fi
}

function uninstall_tiller() {
  helm reset --force
  kubectl delete clusterrolebinding tiller-cluster-rule
  kubectl --namespace kube-system delete sa tiller
}

HELMRELEASE_CRD_URL=https://raw.githubusercontent.com/fluxcd/helm-operator/v1.0.0-rc4/deploy/flux-helm-release-crd.yaml

function install_flux_with_helm() {

  kubectl apply -f "$HELMRELEASE_CRD_URL"

  helm install --name flux --wait \
    --namespace "${FLUX_NAMESPACE}" \
    --set image.repository=docker.io/fluxcd/flux \
    --set image.tag=latest \
    --set git.url=ssh://git@gitsrv/git-server/repos/cluster.git \
    --set git.secretName=flux-git-deploy \
    --set git.pollInterval=10s \
    --set git.config.secretName=gitconfig \
    --set git.config.enabled=true \
    --set-string git.config.data="${GITCONFIG}" \
    --set registry.excludeImage=* \
    --set-string ssh.known_hosts="${KNOWN_HOSTS}" \
    "${FLUX_ROOT_DIR}/chart/flux"
}

function uninstall_flux_with_helm() {
  helm delete --purge flux > /dev/null 2>&1
  kubectl delete -f "$HELMRELEASE_CRD_URL" > /dev/null 2>&1
}

fluxctl_install_cmd="fluxctl install --git-url=ssh://git@gitsrv/git-server/repos/cluster.git --git-email=foo"

function install_flux_with_fluxctl() {
  kustomization_dir=${1:-base/flux}
  key_values_varname=${2}
  kubectl -n "${FLUX_NAMESPACE}" create configmap flux-known-hosts --from-file="${E2E_DIR}/fixtures/known_hosts"
  local kustomtmp
  kustomtmp="$(mktemp -d)"
  defer rm -rf "'${kustomtmp}'"
  mkdir -p "${kustomtmp}/base/flux"
  # This generates the base manifests, which we'll then patch with a kustomization
  echo ">>> writing base configuration to ${kustomtmp}/base/flux" >&3
  $fluxctl_install_cmd --namespace "${FLUX_NAMESPACE}" -o "${kustomtmp}/base/flux"
  # Everything goes into one directory, but not everything is
  # necessarily used by the kustomization
  cp -R "${E2E_DIR}"/fixtures/kustom/* "${kustomtmp}/"
  if [ -n "$key_values_varname" ]; then
    fill_in_place_recursively "$key_values_varname" "${kustomtmp}"
  fi
  kubectl apply -f "$HELMRELEASE_CRD_URL"
  kubectl apply -k "${kustomtmp}/${kustomization_dir}" >&3
  kubectl -n "${FLUX_NAMESPACE}" rollout status -w --timeout=30s deployment/flux
}

function uninstall_flux_with_fluxctl() {
  kubectl delete -n "${FLUX_NAMESPACE}" configmap flux-known-hosts
  $fluxctl_install_cmd --namespace "${FLUX_NAMESPACE}" | kubectl delete -f -
  kubectl delete -f "$HELMRELEASE_CRD_URL" > /dev/null 2>&1
}

function install_git_srv() {
  local external_access_result_var=${1}
  local kustomization_dir=${2:-base/gitsrv}
  local gen_dir
  gen_dir=$(mktemp -d)

  ssh-keygen -t rsa -N "" -f "$gen_dir/id_rsa"
  defer rm -rf "'$gen_dir'"
  kubectl create secret generic flux-git-deploy \
    --namespace="${FLUX_NAMESPACE}" \
    --from-file="${FIXTURES_DIR}/known_hosts" \
    --from-file="$gen_dir/id_rsa" \
    --from-file=identity="$gen_dir/id_rsa" \
    --from-file="$gen_dir/id_rsa.pub"

  kubectl apply -n "${FLUX_NAMESPACE}" -k "${E2E_DIR}/fixtures/kustom/${kustomization_dir}" >&3

  # Wait for the git server to be rolled out
  kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/gitsrv

  local git_srv_podname
  git_srv_podname=$(kubectl get pod -n "${FLUX_NAMESPACE}" -l name=gitsrv -o jsonpath="{['items'][0].metadata.name}")
  coproc kubectl port-forward -n "${FLUX_NAMESPACE}" "$git_srv_podname" :22
  local local_port
  read -r local_port <&"${COPROC[0]}"-
  # shellcheck disable=SC2001
  local_port=$(echo "$local_port" | sed 's%.*:\([0-9]*\).*%\1%')
  local ssh_cmd="ssh -o UserKnownHostsFile=/dev/null  -o StrictHostKeyChecking=no -i $gen_dir/id_rsa -p $local_port"
  # Wait for the git server to be ready
  local check_gitsrv_cmd='git ls-remote ssh://git@localhost/git-server/repos/cluster.git master > /dev/null'
  GIT_SSH_COMMAND="${ssh_cmd}" poll_until_true 'gitsrv to be ready' "${check_gitsrv_cmd}" || {
    kubectl -n "${FLUX_NAMESPACE}" logs deployment/gitsrv
    exit 1
  }

  if [ -n "$external_access_result_var" ]; then
    # return the ssh command needed for git, and the PID of the port-forwarding PID into a variable of choice
    eval "${external_access_result_var}=('$ssh_cmd' '$COPROC_PID')"
  else
    kill "${COPROC_PID}" > /dev/null
  fi
}

function uninstall_git_srv() {
  local secret_name=${1:-flux-git-deploy}
  # Silence secret deletion errors since the secret can be missing (deleted by uninstalling Flux)
  kubectl delete -n "${FLUX_NAMESPACE}" secret "$secret_name" &> /dev/null
  kubectl delete -n "${FLUX_NAMESPACE}" -f "${E2E_DIR}/fixtures/kustom/base/gitsrv/gitsrv.yaml"
}

function install_registry() {
  local external_access_result_var=${1}
  local kustomization_dir=${2:-base/registry}

  kubectl apply -n "${FLUX_NAMESPACE}" -k "${E2E_DIR}/fixtures/kustom/${kustomization_dir}" >&3

  # Wait for the registry to be rolled out
  kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/registry

  local registry_podname
  registry_podname=$(kubectl get pod -n "${FLUX_NAMESPACE}" -l name=registry -o jsonpath="{['items'][0].metadata.name}")
  coproc kubectl port-forward -n "${FLUX_NAMESPACE}" "$registry_podname" :5000
  local local_port
  read -r local_port <&"${COPROC[0]}"-
  # shellcheck disable=SC2001
  local_port=$(echo "$local_port" | sed 's%.*:\([0-9]*\).*%\1%')

  if [ -n "$external_access_result_var" ]; then
    # return the registry local port, and the PID of the port-forwarding PID into a variable of choice
    eval "${external_access_result_var}=('$local_port' '$COPROC_PID')"
  else
    kill "${COPROC_PID}" > /dev/null
  fi
}

function uninstall_registry() {
  kubectl delete -n "${FLUX_NAMESPACE}" -f "${E2E_DIR}/fixtures/kustom/base/registry/registry.yaml"
}
