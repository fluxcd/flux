#!/usr/bin/env bash

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
  local eol=$'\n'
  # Use the local Flux image instead of the latest release, use a poll interval of 10s
  # (to make tests quicker) and disable registry polling (to avoid overloading kind)
  $fluxctl_install_cmd --namespace "${FLUX_NAMESPACE}" |
    sed 's%docker\.io/fluxcd/flux:.*%fluxcd/flux:latest%' |
    sed "s%--git-email=foo%--git-email=foo\\$eol        - --git-poll-interval=10s%" |
    sed "s%--git-email=foo%--git-email=foo\\$eol        - --sync-interval=10s%" |
    sed "s%--git-email=foo%--git-email=foo\\$eol        - --registry-exclude-image=\*%" |
    kubectl apply -f -
  kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/flux
  # Add the known hosts file manually (it's much easier than editing the manifests to add a volume)
  local flux_podname
  flux_podname=$(kubectl get pod -n flux-e2e -l name=flux -o jsonpath="{['items'][0].metadata.name}")
  kubectl exec -n "${FLUX_NAMESPACE}" "${flux_podname}" -- sh -c "echo '${KNOWN_HOSTS}' > /root/.ssh/known_hosts"
}

function uninstall_flux_with_fluxctl() {
  $fluxctl_install_cmd --namespace "${FLUX_NAMESPACE}" | kubectl delete -f -
}

function generate_ssh_secret() {
  local secret_name=${1:-flux-git-deploy}
  local gen_dir
  gen_dir=$(mktemp -d)

  ssh-keygen -t rsa -N "" -f "$gen_dir/id_rsa"
  kubectl create secret generic "$secret_name" \
    --namespace="${FLUX_NAMESPACE}" \
    --from-file="${FIXTURES_DIR}/known_hosts" \
    --from-file="$gen_dir/id_rsa" \
    --from-file=identity="$gen_dir/id_rsa" \
    --from-file="$gen_dir/id_rsa.pub"
  rm -rf "$gen_dir"
  echo "$secret_name"
}

function delete_generated_ssh_secret() {
  local secret_name=${1:-flux-git-deploy}
  kubectl delete -n "${FLUX_NAMESPACE}" secret "$secret_name"
}

function install_git_srv() {
  local secret_name=${1:-flux-git-deploy}

  sed "s/\$GIT_SECRET_NAME/$secret_name/" <"${E2E_DIR}/fixtures/gitsrv.yaml" | kubectl apply -n "${FLUX_NAMESPACE}" -f -

  kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/gitsrv
}

function uninstall_git_srv() {
  kubectl delete -n "${FLUX_NAMESPACE}" -f "${E2E_DIR}/fixtures/gitsrv.yaml"
}
