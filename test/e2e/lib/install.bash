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
--set helmOperator.create=true `# just needed to add the HelmRelease CRD`\
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

fluxctl_install_cmd="fluxctl install --namespace "${FLUX_NAMESPACE}" --git-url=ssh://git@gitsrv/git-server/repos/cluster.git --git-email=foo"

function install_flux_with_fluxctl() {
  local eol=$'\n'
  # Use the local Flux image instead of the latest release, use a poll interval of 10s
  # (to make tests quicker) and disable registry polling (to avoid overloading kind)
  $fluxctl_install_cmd \
    --flux-image="fluxcd/flux:latest" \
    --extra-flux-arguments="--git-poll-interval=10s,--sync-interval=10s,--registry-exclude-image=\*" | kubectl apply -f -
  kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/flux
  # Add the known hosts file manually (it's much easier than editing the manifests to add a volume)
  local flux_podname=$(kubectl get pod -n flux-e2e -l name=flux -o jsonpath="{['items'][0].metadata.name}")
  kubectl exec -n "${FLUX_NAMESPACE}" "${flux_podname}" -- sh -c "echo '$(cat ${FIXTURES_DIR}/known_hosts)' > /root/.ssh/known_hosts"
}

function uninstall_flux_with_fluxctl() {
  $fluxctl_install_cmd | kubectl delete -f -
}

function install_git_srv() {
  kubectl apply -n "${FLUX_NAMESPACE}" -f "${E2E_DIR}/fixtures/gitsrv.yaml"
  kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/gitsrv
}

function uninstall_git_srv() {
  kubectl delete -n "${FLUX_NAMESPACE}" -f "${E2E_DIR}/fixtures/gitsrv.yaml"
}
