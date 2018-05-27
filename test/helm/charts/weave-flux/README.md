# Weave Flux OSS

Flux is a tool that automatically ensures that the state of a cluster matches what is specified in version control.
It is most useful when used as a deployment tool at the end of a Continuous Delivery pipeline. Flux will make sure that your new container images and config changes are propagated to the cluster.

## Introduction

This chart bootstraps a [Weave Flux](https://github.com/weaveworks/flux) deployment on 
a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.7+

## Installing the Chart

To install the chart with the release name `cd`:

```console
$ helm install --name cd \
--set git.url=git@github.com:weaveworks/flux-example \
--namespace flux \
./charts/weave-flux
```

To install Flux with the Helm operator:

```console
$ helm install --name cd \
--set git.url=git@github.com:stefanprodan/weave-flux-helm-demo \
--set git.user="Stefan Prodan" \
--set git.email="stefan.prodan@gmail.com" \
--set helmOperator.create=true \
--namespace flux \
./charts/weave-flux
```

Be aware that the Helm operator is alpha quality, DO NOT use it on a production cluster.

The [configuration](#configuration) section lists the parameters that can be configured during installation.

### Setup Git deploy 

At startup Flux generates a SSH key and logs the public key. 
Find the SSH public key with:

```bash
export FLUX_POD=$(kubectl get pods --namespace flux -l "app=weave-flux,release=cd" -o jsonpath="{.items[0].metadata.name}")
kubectl -n flux logs $FLUX_POD | grep identity.pub | cut -d '"' -f2 | sed 's/.\{2\}$//'
```

In order to sync your cluster state with GitHub you need to copy the public key and 
create a deploy key with write access on your GitHub repository.

## Uninstalling the Chart

To uninstall/delete the `cd` deployment:

```console
$ helm delete --purge cd
```

The command removes all the Kubernetes components associated with the chart and deletes the release. 
You should also remove the deploy key from your GitHub repository.

## Configuration

The following tables lists the configurable parameters of the Weave Flux chart and their default values.

| Parameter                       | Description                                | Default                                                    |
| ------------------------------- | ------------------------------------------ | ---------------------------------------------------------- |
| `image.repository` | Image repository | `quay.io/weaveworks/flux` 
| `image.tag` | Image tag | `1.2.5` 
| `image.pullPoliwell cy` | Image pull policy | `IfNotPresent` 
| `resources` | CPU/memory resource requests/limits | None 
| `rbac.create` | If `true`, create and use RBAC resources | `true`
| `serviceAccount.create` | If `true`, create a new service account | `true`
| `serviceAccount.name` | Service account to be used | `weave-flux`
| `service.type` | Service type to be used | `ClusterIP`
| `service.port` | Service port to be used | `3030`
| `git.url` | URL of git repo with Kubernetes manifests | None
| `git.branch` | Branch of git repo to use for Kubernetes manifests | `master`
| `git.path` | Path within git repo to locate Kubernetes manifests (relative path) | None
| `git.user` | Username to use as git committer | `Weave Flux`
| `git.email` | Email to use as git committer | `support@weave.works`
| `git.chartsPath` | Path within git repo to locate Helm charts (relative path) | `charts`
| `git.pollInterval` | Period at which to poll git repo for new commits | `30s`
| `helmOperator.create` | If `true`, install the Helm operator | `false`
| `helmOperator.repository` | Helm operator image repository | `quay.io/weaveworks/helm-operator` 
| `helmOperator.tag` | Helm operator image tag | `master-6f427cb` 
| `helmOperator.pullPolicy` | Helm operator image pull policy | `IfNotPresent` 
| `token` | Weave Cloud service token | None 

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example:

```console
$ helm upgrade --install --wait cd \
--set git.url=git@github.com:stefanprodan/podinfo \
--set git.path=deploy/auto-scaling \
--namespace flux \
./charts/weave-flux
```

## Upgrade

Update Weave Flux version with:

```console
helm upgrade --reuse-values cd \
--set image.tag=1.2.6 \
./charts/weave-flux
```



