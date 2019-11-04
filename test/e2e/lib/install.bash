#!/usr/bin/env bash

# shellcheck disable=SC1090
source "${E2E_DIR}/lib/defer.bash"

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

function install_flux_with_helm() {
  local create_crds='true'
  if kubectl get crd fluxhelmreleases.helm.integrations.flux.weave.works helmreleases.flux.weave.works > /dev/null 2>&1; then
    # CRDs existed, don't try to create them
    echo 'CRDs existed, setting helmOperator.createCRD=false'
    create_crds='false'
  fi

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
    --set helmOperator.create=true \
    --set helmOperator.git.secretName=flux-git-deploy \
    --set helmOperator.createCRD="${create_crds}" \
    --set registry.excludeImage=* \
    --set-string ssh.known_hosts="${KNOWN_HOSTS}" \
    "${FLUX_ROOT_DIR}/chart/flux"
}

function uninstall_flux_with_helm() {
  helm delete --purge flux > /dev/null 2>&1
  kubectl delete crd helmreleases.flux.weave.works > /dev/null 2>&1
}

fluxctl_install_cmd="fluxctl install --git-url=ssh://git@gitsrv/git-server/repos/cluster.git --git-email=foo"

function install_flux_with_fluxctl() {
  local kustomtmp
  kustomtmp="$(mktemp -d)"
  # This generates the base descriptions, which we'll then patch with a kustomization
  $fluxctl_install_cmd --namespace "${FLUX_NAMESPACE}" -o "${kustomtmp}" 2>&3
  cp ${E2E_DIR}/fixtures/{kustomization,e2e_patch}.yaml "${kustomtmp}/"
  kubectl apply -k "${kustomtmp}" >&3
  kubectl -n "${FLUX_NAMESPACE}" rollout status -w --timeout=30s deployment/flux
  # Add the known hosts file manually (it's much easier than editing the manifests to add a volume)
  local flux_podname
  flux_podname=$(kubectl get pod -n "${FLUX_NAMESPACE}" -l name=flux -o jsonpath="{['items'][0].metadata.name}")
  kubectl exec -n "${FLUX_NAMESPACE}" "${flux_podname}" -- sh -c "mkdir -p /root/.ssh; echo '${KNOWN_HOSTS}' > /root/.ssh/known_hosts" >&3
}

function uninstall_flux_with_fluxctl() {
  $fluxctl_install_cmd --namespace "${FLUX_NAMESPACE}" | kubectl delete -f -
}

flux_gpg_helm_template="helm template --name flux-gpg
    --set image.repository=docker.io/fluxcd/flux
    --set image.tag=latest
    --set git.url=ssh://git@gitsrv/git-server/repos/cluster.git
    --set git.secretName=flux-git-deploy
    --set git.pollInterval=10s
    --set git.config.secretName=gitconfig
    --set git.config.enabled=true
    --set registry.excludeImage=*"

function install_flux_gpg() {
  local key_id=${1}
  local git_verify=${2:-false}
  local gpg_secret_name=${3:-flux-gpg-signing-key}

  if [ -z "$key_id" ]; then
    echo "no key ID provided" >&2
    exit 1
  fi

  $flux_gpg_helm_template \
    --namespace "${FLUX_NAMESPACE}" \
    --set-string git.config.data="${GITCONFIG}" \
    --set-string ssh.known_hosts="${KNOWN_HOSTS}" \
    --set-string git.signingKey="$key_id" \
    --set-string git.verifySignatures="$git_verify" \
    --set-string gpgKeys.secretName="$gpg_secret_name" \
    "${FLUX_ROOT_DIR}/chart/flux" |
    kubectl --namespace "${FLUX_NAMESPACE}" apply -f - >&3
}

function uninstall_flux_gpg() {
  $flux_gpg_helm_template \
    --namespace "${FLUX_NAMESPACE}" \
    --set-string git.config.data="${GITCONFIG}" \
    --set-string ssh.known_hosts="${KNOWN_HOSTS}" \
    "${FLUX_ROOT_DIR}/chart/flux" |
    kubectl --namespace "${FLUX_NAMESPACE}" delete -f - >&3
}

function install_git_srv() {
  local git_secret_name=${1:-flux-git-deploy}
  local external_access_result_var=${2}
  local gpg_enable=${3:-false}
  local gpg_secret_name=${4:-flux-gpg-signing-key}
  local gen_dir
  gen_dir=$(mktemp -d)

  ssh-keygen -t rsa -N "" -f "$gen_dir/id_rsa"
  defer rm -rf "$gen_dir"
  kubectl create secret generic "$git_secret_name" \
    --namespace="${FLUX_NAMESPACE}" \
    --from-file="${FIXTURES_DIR}/known_hosts" \
    --from-file="$gen_dir/id_rsa" \
    --from-file=identity="$gen_dir/id_rsa" \
    --from-file="$gen_dir/id_rsa.pub"

  local template="${E2E_DIR}/fixtures/gitsrv.yaml"
  if [ "$gpg_enable" == "true" ]; then
    template="${E2E_DIR}/fixtures/gitsrv-gpg.yaml"
  fi

  (
    export GIT_SECRET_NAME=$git_secret_name
    export GPG_SECRET_NAME=$gpg_secret_name

    envsubst < "$template" | kubectl apply -n "${FLUX_NAMESPACE}" -f - >&3
  )

  # Wait for the git server to be ready
  kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/gitsrv

  if [ -n "$external_access_result_var" ]; then
    local git_srv_podname
    git_srv_podname=$(kubectl get pod -n "${FLUX_NAMESPACE}" -l name=gitsrv -o jsonpath="{['items'][0].metadata.name}")
    coproc kubectl port-forward -n "${FLUX_NAMESPACE}" "$git_srv_podname" :22
    local local_port
    read -r local_port <&"${COPROC[0]}"-
    # shellcheck disable=SC2001
    local_port=$(echo "$local_port" | sed 's%.*:\([0-9]*\).*%\1%')
    local ssh_cmd="ssh -o UserKnownHostsFile=/dev/null  -o StrictHostKeyChecking=no -i $gen_dir/id_rsa -p $local_port"
    # return the ssh command needed for git, and the PID of the port-forwarding PID into a variable of choice
    eval "${external_access_result_var}=('$ssh_cmd' '$COPROC_PID')"
  fi
}

function uninstall_git_srv() {
  local secret_name=${1:-flux-git-deploy}
  # Silence secret deletion errors since the secret can be missing (deleted by uninstalling Flux)
  kubectl delete -n "${FLUX_NAMESPACE}" secret "$secret_name" &> /dev/null
  kubectl delete -n "${FLUX_NAMESPACE}" -f "${E2E_DIR}/fixtures/gitsrv.yaml"
}
