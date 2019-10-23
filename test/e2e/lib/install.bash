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
--set git.secretName=ssh-git \
--set git.pollInterval=30s \
--set git.config.secretName=gitconfig \
--set git.config.enabled=true \
--set-string git.config.data="${GITCONFIG}" \
--set helmOperator.create=true `# just needed to add the HelmRelease CRD`\
--set helmOperator.git.secretName=ssh-git \
--set helmOperator.createCRD="${create_crds}" \
--set registry.excludeImage=* \
--set-string ssh.known_hosts="${KNOWN_HOSTS}" \
"${FLUX_ROOT_DIR}/chart/flux"

}

function uninstall_flux_with_helm() {
  helm delete --purge flux > /dev/null 2>&1
  kubectl delete crd helmreleases.flux.weave.works > /dev/null 2>&1
}

function install_git_srv() {
  kubectl apply -n "${FLUX_NAMESPACE}" -f "${E2E_DIR}/fixtures/gitsrv.yaml"
  kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/gitsrv
}

function uninstall_git_srv() {
  kubectl delete -n "${FLUX_NAMESPACE}" -f "${E2E_DIR}/fixtures/gitsrv.yaml"
}