# Flux

Flux is a tool that automatically ensures that the state of a cluster matches the config in git. 
It uses an operator in the cluster to trigger deployments inside Kubernetes, which means you don't need a separate CD tool. 
It monitors all relevant image repositories, detects new images, triggers deployments and updates the desired running
configuration based on that (and a configurable policy).

## Introduction

This chart bootstraps a [Flux](https://github.com/weaveworks/flux) deployment on 
a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.9+

## Installing the Chart

Add the weaveworks repo:

```
helm repo add weaveworks https://weaveworks.github.io/flux
```

To install the chart with the release name `flux`:

```console
$ helm install --name flux \
--set git.url=git@github.com:weaveworks/flux-example \
--namespace flux \
weaveworks/flux
```

To connect Flux to a Weave Cloud instance:

```console
helm install --name flux \
--set token=YOUR_WEAVE_CLOUD_SERVICE_TOKEN \
--namespace flux \
weaveworks/flux
```

To install Flux with the Helm operator (alpha version):

```console
$ helm install --name flux \
--set git.url=git@github.com:weaveworks/flux-helm-test \
--set helmOperator.create=true \
--namespace flux \
weaveworks/flux
```

The [configuration](#configuration) section lists the parameters that can be configured during installation.

### Setup Git deploy 

At startup Flux generates a SSH key and logs the public key. 
Find the SSH public key with:

```bash
kubectl -n flux logs deployment/flux | grep identity.pub
```

In order to sync your cluster state with GitHub you need to copy the public key and 
create a deploy key with write access on your GitHub repository.
Go to _Settings > Deploy keys_ click on _Add deploy key_, check 
_Allow write access_, paste the Flux public key and click _Add key_.

## Uninstalling the Chart

To uninstall/delete the `flux` deployment:

```console
$ helm delete --purge flux
```

The command removes all the Kubernetes components associated with the chart and deletes the release. 
You should also remove the deploy key from your GitHub repository.

## Configuration

The following tables lists the configurable parameters of the Weave Flux chart and their default values.

| Parameter                       | Description                                | Default                                                    |
| ------------------------------- | ------------------------------------------ | ---------------------------------------------------------- |
| `image.repository` | Image repository | `quay.io/weaveworks/flux` 
| `image.tag` | Image tag | `1.3.1` 
| `image.pullPolicy` | Image pull policy | `IfNotPresent` 
| `resources` | CPU/memory resource requests/limits | None 
| `rbac.create` | If `true`, create and use RBAC resources | `true`
| `serviceAccount.create` | If `true`, create a new service account | `true`
| `serviceAccount.name` | Service account to be used | `flux`
| `service.type` | Service type to be used (exposing the Flux API outside of the cluster is not advised) | `ClusterIP`
| `service.port` | Service port to be used | `3030`
| `git.url` | URL of git repo with Kubernetes manifests | None
| `git.branch` | Branch of git repo to use for Kubernetes manifests | `master`
| `git.path` | Path within git repo to locate Kubernetes manifests (relative path) | None
| `git.user` | Username to use as git committer | `Weave Flux`
| `git.email` | Email to use as git committer | `support@weave.works`
| `git.chartsPath` | Path within git repo to locate Helm charts (relative path) | `charts`
| `git.pollInterval` | Period at which to poll git repo for new commits | `30s`
| `ssh.known_hosts`  | The contents of an SSH `known_hosts` file, if you need to supply host key(s) |
| `helmOperator.create` | If `true`, install the Helm operator | `false`
| `helmOperator.repository` | Helm operator image repository | `quay.io/weaveworks/helm-operator` 
| `helmOperator.tag` | Helm operator image tag | `0.1.0-alpha` 
| `helmOperator.pullPolicy` | Helm operator image pull policy | `IfNotPresent` 
| `token` | Weave Cloud service token | None 

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example:

```console
$ helm upgrade --install --wait flux \
--set git.url=git@github.com:stefanprodan/podinfo \
--set git.path=deploy/auto-scaling \
--namespace flux \
weaveworks/flux
```

## Upgrade

Update Weave Flux version with:

```console
helm upgrade --reuse-values flux \
--set image.tag=1.3.2 \
weaveworks/flux
```



