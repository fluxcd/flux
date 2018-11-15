# Flux

Flux is a tool that automatically ensures that the state of a cluster matches the config in git.
It uses an operator in the cluster to trigger deployments inside Kubernetes, which means you don't need a separate CD tool.
It monitors all relevant image repositories, detects new images, triggers deployments and updates the desired running
configuration based on that (and a configurable policy).

## Introduction

This chart bootstraps a [Flux](https://github.com/weaveworks/flux) deployment on
a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

### Kubernetes

Kubernetes >= v1.9 is recommended. Kubernetes v1.8 (the first to support
Custom Resources) appears to have problems with repeated application of
custom resources (see https://github.com/kubernetes/kubernetes/issues/53379).
This means fluxd can fail to apply changes to FluxHelmRelease resources.

### Helm

Tiller should be running in the cluster, though
[helm-operator](../../site/helm-operator.md) will wait
until it can find one.

# Git repo

 - One repo containing both desired release state information and Charts
   themselves.
 - Release state information in the form of Custom Resources manifests is
   located under a particular path ("releaseconfig" by default; can be
   overriden).
 - Charts are colocated under another path ("charts" by default; can be
   overriden). Charts are subdirectories under the charts path.
 - Custom Resource namespace reflects where the release should be done.
   Both the Helm release and its corresponding Custom Resource will
   live in this namespace.
 - Example of a test repo: https://github.com/weaveworks/flux-helm-test

## Installation

We put together a simple [Get Started
guide](../../site/helm-get-started.md) which takes about 5-10 minutes to follow.
You will have a fully working Flux installation deploying workloads to your
cluster.

## Installing Flux using Helm

### Installing the Chart

Add the weaveworks repo:

```sh
helm repo add weaveworks https://weaveworks.github.io/flux
```

#### To install the chart with the release name `flux`:

```sh
$ helm install --name flux \
--set git.url=ssh://git@github.com/weaveworks/flux-example \
--namespace flux \
weaveworks/flux
```

#### To connect Flux to a Weave Cloud instance:

```sh
helm install --name flux \
--set token=YOUR_WEAVE_CLOUD_SERVICE_TOKEN \
--namespace flux \
weaveworks/flux
```

#### To install Flux with the Helm operator:

```sh
$ helm install --name flux \
--set git.url=ssh://git@github.com/weaveworks/flux-helm-test \
--set helmOperator.create=true \
--namespace flux \
weaveworks/flux
```

#### To install Flux with a private git host:

When using a private git host, setting the `ssh.known_hosts` variable 
is required for enabling successful key matches because `StrictHostKeyChecking` 
is enabled during flux git daemon operations.

By setting the `ssh.known_hosts` variable, a configmap will be created
called `flux-ssh-config` which in turn will be mounted into a volume named
`sshdir` at `/root/.ssh/known_hosts`.

* Get the `ssh.known_hosts` keys by running the following command:

```sh
ssh-keyscan <your_git_host_domain>
```

To prevent a potential man-in-the-middle attack, one should
verify the ssh keys acquired through the `ssh-keyscan` match expectations
using an alternate mechanism.

* Start flux and flux helm operator:

  - Using a string for setting `known_hosts`

    ```sh
    YOUR_GIT_HOST=your_git_host.example.com
    KNOWN_HOSTS='domain ssh-rsa line1
    domain ecdsa-sha2-line2
    domain ssh-ed25519 line3'

    helm install \
    --name flux \
    --set helmOperator.create=true \
    --set git.url="ssh://git@${YOUR_GIT_HOST}:weaveworks/flux-helm-test.git" \
    --set-string ssh.known_hosts="${KNOWN_HOSTS}" \
    --namespace flux \
    chart/flux
    ```

  - Using a file for setting `known_hosts`

    Copy known_hosts keys into a temporary file `/tmp/flux_known_hosts`

    ```sh
    YOUR_GIT_HOST=your_git_host.example.com

    helm install \
    --name flux \
    --set helmOperator.create=true \
    --set git.url="ssh://git@${YOUR_GIT_HOST}:weaveworks/flux-helm-test.git" \
    --set-file ssh.known_hosts=/tmp/flux_known_hosts \
    --namespace flux \
    chart/flux
    ```

The [configuration](#configuration) section lists all the parameters that can be configured during installation.

#### Setup Git deploy

At startup Flux generates a SSH key and logs the public key.
Find the SSH public key with:

```sh
kubectl -n flux logs deployment/flux | grep identity.pub | cut -d '"' -f2
```

In order to sync your cluster state with GitHub you need to copy the public key and
create a deploy key with write access on your GitHub repository.
Go to _Settings > Deploy keys_ click on _Add deploy key_, check
_Allow write access_, paste the Flux public key and click _Add key_.

### Uninstalling the Chart

To uninstall/delete the `flux` deployment:

```sh
helm delete --purge flux
```

The command removes all the Kubernetes components associated with the chart and deletes the release.
You should also remove the deploy key from your GitHub repository.

### Configuration

The following tables lists the configurable parameters of the Weave Flux chart and their default values.

| Parameter                       | Description                                | Default                                                    |
| ------------------------------- | ------------------------------------------ | ---------------------------------------------------------- |
| `image.repository` | Image repository | `quay.io/weaveworks/flux`
| `image.tag` | Image tag | `<VERSION>`
| `image.pullPolicy` | Image pull policy | `IfNotPresent`
| `resources` | CPU/memory resource requests/limits for Flux | None
| `token` | Weave Cloud service token | None
| `extraEnvs` | Extra environment variables for the Flux pod | `[]`
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
| `git.setAuthor` | If set, the author of git commits will reflect the user who initiated the commit and will differ from the git committer. | `false`
| `git.label` | Label to keep track of sync progress, used to tag the Git branch | `flux-sync`
| `git.ciSkip` | Append "[ci skip]" to commit messages so that CI will skip builds | `false`
| `git.pollInterval` | Period at which to poll git repo for new commits | `5m`
| `git.timeout` | Duration after which git operations time out | `20s`
| `git.secretName` | Kubernetes secret with the SSH private key | None
| `ssh.known_hosts`  | The contents of an SSH `known_hosts` file, if you need to supply host key(s) | None
| `registry.pollInterval` | Period at which to check for updated images | `5m`
| `registry.rps` | Maximum registry requests per second per host | `200`
| `registry.burst` | Maximum number of warmer connections to remote and memcache | `125`
| `registry.trace` |  Output trace of image registry requests to log | `false`
| `registry.insecureHosts` | Use HTTP rather than HTTPS for these image registry domains | None
| `memcached.verbose` | Enable request logging in memcached | `false`
| `memcached.maxItemSize` | Maximum size for one item | `1m`
| `memcached.maxMemory` | Maximum memory to use, in megabytes | `64`
| `memcached.resources` | CPU/memory resource requests/limits for memcached | None
| `helmOperator.create` | If `true`, install the Helm operator | `false`
| `helmOperator.repository` | Helm operator image repository | `quay.io/weaveworks/helm-operator`
| `helmOperator.tag` | Helm operator image tag | `<VERSION>`
| `helmOperator.pullPolicy` | Helm operator image pull policy | `IfNotPresent`
| `helmOperator.updateChartDeps` | Update dependencies for charts | `true`
| `helmOperator.chartsSyncInterval` | Interval at which to check for changed charts | `3m`
| `helmOperator.chartsSyncTimeout` | Timeout when checking for changed charts | `1m`
| `helmOperator.extraEnvs` | Extra environment variables for the Helm operator pod | `[]`
| `helmOperator.git.url` | URL of git repo with Helm charts | `git.url`
| `helmOperator.git.branch` | Branch of git repo to use for Helm charts | `master`
| `helmOperator.git.chartsPath` | Path within git repo to locate Helm charts (relative path) | `charts`
| `helmOperator.git.pollInterval` | Period at which to poll git repo for new commits | `git.pollInterval`
| `helmOperator.git.timeout` | Duration after which git operations time out | `git.timeout`
| `helmOperator.git.secretName` | Kubernetes secret with the SSH private key | None
| `helmOperator.logReleaseDiffs` | Helm operator should log the diff when a chart release diverges (possibly insecure) | `false`
| `helmOperator.tillerNamespace` | Namespace in which the Tiller server can be found | `kube-system`
| `helmOperator.tls.enable` | Enable TLS for communicating with Tiller | `false`
| `helmOperator.tls.verify` | Verify the Tiller certificate, also enables TLS when set to true | `false`
| `helmOperator.tls.secretName` | Name of the secret containing the TLS client certificates for communicating with Tiller | `helm-client-certs`
| `helmOperator.tls.keyFile` | Name of the key file within the k8s secret | `tls.key`
| `helmOperator.tls.certFile` | Name of the certificate file within the k8s secret | `tls.crt`
| `helmOperator.tls.caContent` | Certificate Authority content used to validate the Tiller server certificate | None
| `helmOperator.resources` | CPU/memory resource requests/limits for Helm operator | None

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example:

```sh
$ helm upgrade --install --wait flux \
--set git.url=ssh://git@github.com/stefanprodan/podinfo \
--set git.path=deploy/auto-scaling,deploy/local-storage \
--namespace flux \
weaveworks/flux
```

### Upgrade

Update Weave Flux version with:

```sh
helm upgrade --reuse-values flux \
--set image.tag=1.7.1 \
weaveworks/flux
```
