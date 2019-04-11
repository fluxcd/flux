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

Kubernetes >= v1.10 is recommended. Kubernetes v1.8 (the first to support
Custom Resources) appears to have problems with repeated application of
custom resources (see https://github.com/kubernetes/kubernetes/issues/53379).
This means fluxd can fail to apply changes to HelmRelease resources.

### Helm

Tiller should be running in the cluster, though
[helm-operator](../../site/helm-operator.md) will wait
until it can find one.

# Git repo

 - One repo containing cluster config (i.e., Kubernetes YAMLs) and zero or more git repos containing Charts themselves.
 - Charts can be co-located with config in the git repo, or be from Helm repositories.
 - Custom Resource namespace reflects where the release should be done.
   Both the Helm release and its corresponding Custom Resource will
   live in this namespace.
 - Example of a test repo: https://github.com/weaveworks/flux-get-started

## Installation

We put together a simple [Get Started
guide](../../site/helm-get-started.md) which takes about 5-10 minutes to follow.
You will have a fully working Flux installation deploying workloads to your cluster.

## Installing Flux using Helm

### Installing the Chart

Add the weaveworks repo:

```sh
helm repo add weaveworks https://weaveworks.github.io/flux
```

#### To install the chart with the release name `flux`

Replace `weaveworks/flux-get-started` with your own git repository and run helm install:

```sh
$ helm install --name flux \
--set git.url=git@github.com:weaveworks/flux-get-started \
--namespace flux \
weaveworks/flux
```

#### To connect Flux to a Weave Cloud instance:

```sh
helm install --name flux \
--set git.url=git@github.com:weaveworks/flux-get-started \
--set token=YOUR_WEAVE_CLOUD_SERVICE_TOKEN \
--namespace flux \
weaveworks/flux
```

#### To install Flux with the Helm operator:

Apply the Helm Release CRD:

```sh
kubectl apply -f https://raw.githubusercontent.com/weaveworks/flux/master/deploy-helm/flux-helm-release-crd.yaml
```

Install Flux with Helm:

```sh
$ helm install --name flux \
--set git.url=git@github.com:weaveworks/flux-get-started \
--set helmOperator.create=true \
--set helmOperator.createCRD=false \
--namespace flux \
weaveworks/flux
```

#### To install Flux with a private git host:

When using a private git host, setting the `ssh.known_hosts` variable
is required for enabling successful key matches because `StrictHostKeyChecking`
is enabled during Flux git daemon operations.

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

* Start Flux and Flux helm operator:

  - Using a string for setting `known_hosts`

    ```sh
    YOUR_GIT_HOST=your_git_host.example.com
    YOUR_GIT_USER=your_git_user
    KNOWN_HOSTS='domain ssh-rsa line1
    domain ecdsa-sha2-line2
    domain ssh-ed25519 line3'

    helm install \
    --name flux \
    --set helmOperator.create=true \
    --set helmOperator.createCRD=false \
    --set git.url="git@${YOUR_GIT_HOST}:${YOUR_GIT_USER}/flux-get-started" \
    --set-string ssh.known_hosts="${KNOWN_HOSTS}" \
    --namespace flux \
    chart/flux
    ```

  - Using a file for setting `known_hosts`

    Copy known_hosts keys into a temporary file `/tmp/flux_known_hosts`

    ```sh
    YOUR_GIT_HOST=your_git_host.example.com
    YOUR_GIT_USER=your_git_user

    helm install \
    --name flux \
    --set helmOperator.create=true \
    --set helmOperator.createCRD=false \
    --set git.url="git@${YOUR_GIT_HOST}:${YOUR_GIT_USER}/flux-get-started" \
    --set-file ssh.known_hosts=/tmp/flux_known_hosts \
    --namespace flux \
    chart/flux
    ```

The [configuration](#configuration) section lists all the parameters that can be configured during installation.

#### Setup Git deploy

At startup Flux generates a SSH key and logs the public key.
Find the SSH public key by installing [fluxctl](../../site/fluxctl.md) and
running:

```sh
fluxctl identity
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

| Parameter                                         | Default                                              | Description
| -----------------------------------------------   | ---------------------------------------------------- | ---
| `image.repository`                                | `quay.io/weaveworks/flux`                            | Image repository
| `image.tag`                                       | `<VERSION>`                                          | Image tag
| `replicaCount`                                    | `1`                                                  | Number of Flux pods to deploy, more than one is not desirable.
| `image.pullPolicy`                                | `IfNotPresent`                                       | Image pull policy
| `image.pullSecret`                                | `None`                                               | Image pull secret
| `resources.requests.cpu`                          | `50m`                                                | CPU resource requests for the Flux deployment
| `resources.requests.memory`                       | `64Mi`                                               | Memory resource requests for the Flux deployment
| `resources.limits`                                | `None`                                               | CPU/memory resource limits for the Flux deployment
| `nodeSelector`                                    | `{}`                                                 | Node Selector properties for the Flux deployment
| `tolerations`                                     | `[]`                                                 | Tolerations properties for the Flux deployment
| `affinity`                                        | `{}`                                                 | Affinity properties for the Flux deployment
| `token`                                           | `None`                                               | Weave Cloud service token
| `extraEnvs`                                       | `[]`                                                 | Extra environment variables for the Flux pod(s)
| `rbac.create`                                     | `true`                                               | If `true`, create and use RBAC resources
| `serviceAccount.create`                           | `true`                                               | If `true`, create a new service account
| `serviceAccount.name`                             | `flux`                                               | Service account to be used
| `service.type`                                    | `ClusterIP`                                          | Service type to be used (exposing the Flux API outside of the cluster is not advised)
| `service.port`                                    | `3030`                                               | Service port to be used
| `git.url`                                         | `None`                                               | URL of git repo with Kubernetes manifests
| `git.branch`                                      | `master`                                             | Branch of git repo to use for Kubernetes manifests
| `git.path`                                        | `None`                                               | Path within git repo to locate Kubernetes manifests (relative path)
| `git.user`                                        | `Weave Flux`                                         | Username to use as git committer
| `git.email`                                       | `support@weave.works`                                | Email to use as git committer
| `git.setAuthor`                                   | `false`                                              | If set, the author of git commits will reflect the user who initiated the commit and will differ from the git committer.
| `git.signingKey`                                  | `None`                                               | If set, commits will be signed with this GPG key
| `git.label`                                       | `flux-sync`                                          | Label to keep track of sync progress, used to tag the Git branch
| `git.ciSkip`                                      | `false`                                              | Append "[ci skip]" to commit messages so that CI will skip builds
| `git.pollInterval`                                | `5m`                                                 | Period at which to poll git repo for new commits
| `git.timeout`                                     | `20s`                                                | Duration after which git operations time out
| `git.secretName`                                  | `None`                                               | Kubernetes secret with the SSH private key. Superceded by `helmOperator.git.secretName` if set.
| `gpgKeys.secretName`                              | `None`                                               | Kubernetes secret with GPG keys the Flux daemon should import
| `ssh.known_hosts`                                 | `None`                                               | The contents of an SSH `known_hosts` file, if you need to supply host key(s)
| `registry.pollInterval`                           | `5m`                                                 | Period at which to check for updated images
| `registry.rps`                                    | `200`                                                | Maximum registry requests per second per host
| `registry.burst`                                  | `125`                                                | Maximum number of warmer connections to remote and memcache
| `registry.trace`                                  | `false`                                              |  Output trace of image registry requests to log
| `registry.insecureHosts`                          | `None`                                               | Use HTTP rather than HTTPS for the image registry domains
| `registry.cacheExpiry`                            | `None`                                               | Duration to keep cached image info (deprecated)
| `registry.excludeImage`                           | `None`                                               | Do not scan images that match these glob expressions; if empty, 'k8s.gcr.io/*' images are excluded
| `registry.ecr.region`                             | `None`                                               | Restrict ECR scanning to these AWS regions; if empty, only the cluster's region will be scanned
| `registry.ecr.includeId`                          | `None`                                               | Restrict ECR scanning to these AWS account IDs; if empty, all account IDs that aren't excluded may be scanned
| `registry.ecr.excludeId`                          | `602401143452`                                       | Do not scan ECR for images in these AWS account IDs; the default is to exclude the EKS system account
| `registry.ecr.require`                            | `false`                                              | Refuse to start if the AWS API is not available
| `registry.acr.enabled`                            | `false`                                              | Mount `azure.json` via HostPath into the Flux Pod, enabling Flux to use AKS's service principal for ACR authentication
| `registry.acr.hostPath`                           | `/etc/kubernetes/azure.json`                         | Alternative location of `azure.json` on the host
| `registry.dockercfg.enabled`                      | `false`                                              | Mount `config.json` via Secret into the Flux Pod, enabling Flux to use a custom docker config file
| `registry.dockercfg.secretName`                   | `None`                                               | Kubernetes secret with the docker config.json
| `registry.dockercfg.configFileName`               | `/dockercfg/config.json`                             | Alternative path/name of the docker config.json
| `memcached.verbose`                               | `false`                                              | Enable request logging in memcached
| `memcached.maxItemSize`                           | `5m`                                                 | Maximum size for one item
| `memcached.maxMemory`                             | `128`                                                | Maximum memory to use, in megabytes
| `memcached.pullSecret`                            | `None`                                               | Image pull secret
| `memcached.repository`                            | `memcached`                                          | Image repository
| `memcached.resources`                             | `None`                                               | CPU/memory resource requests/limits for memcached
| `helmOperator.create`                             | `false`                                              | If `true`, install the Helm operator
| `helmOperator.createCRD`                          | `true`                                               | Create the `v1beta1` and `v1alpha2` Flux CRDs. Dependent on `helmOperator.create=true`
| `helmOperator.repository`                         | `quay.io/weaveworks/helm-operator`                   | Helm operator image repository
| `helmOperator.tag`                                | `<VERSION>`                                          | Helm operator image tag
| `helmOperator.replicaCount`                       | `1`                                                  | Number of helm operator pods to deploy, more than one is not desirable.
| `helmOperator.pullPolicy`                         | `IfNotPresent`                                       | Helm operator image pull policy
| `helmOperator.pullSecret`                         | `None`                                               | Image pull secret
| `helmOperator.updateChartDeps`                    | `true`                                               | Update dependencies for charts
| `helmOperator.git.pollInterval`                   | `git.pollInterval`                                   | Period on which to poll git chart sources for changes
| `helmOperator.git.timeout`                        | `git.timeout`                                        | Duration after which git operations time out
| `helmOperator.git.secretName`                     | `None`                                               | The name of the kubernetes secret with the SSH private key, supercedes `git.secretName`
| `helmOperator.chartsSyncInterval`                 | `3m`                                                 | Interval at which to check for changed charts
| `helmOperator.extraEnvs`                          | `[]`                                                 | Extra environment variables for the Helm operator pod
| `helmOperator.logReleaseDiffs`                    | `false`                                              | Helm operator should log the diff when a chart release diverges (possibly insecure)
| `helmOperator.allowNamespace`                     | `None`                                               | If set, this limits the scope to a single namespace. If not specified, all namespaces will be watched
| `helmOperator.tillerNamespace`                    | `kube-system`                                        | Namespace in which the Tiller server can be found
| `helmOperator.tls.enable`                         | `false`                                              | Enable TLS for communicating with Tiller
| `helmOperator.tls.verify`                         | `false`                                              | Verify the Tiller certificate, also enables TLS when set to true
| `helmOperator.tls.secretName`                     | `helm-client-certs`                                  | Name of the secret containing the TLS client certificates for communicating with Tiller
| `helmOperator.tls.keyFile`                        | `tls.key`                                            | Name of the key file within the k8s secret
| `helmOperator.tls.certFile`                       | `tls.crt`                                            | Name of the certificate file within the k8s secret
| `helmOperator.tls.caContent`                      | `None`                                               | Certificate Authority content used to validate the Tiller server certificate
| `helmOperator.tls.hostname`                       | `None`                                               | The server name used to verify the hostname on the returned certificates from the Tiller server
| `helmOperator.configureRepositories.enable`       | `false`                                              | Enable volume mount for a `repositories.yaml` configuration file and respository cache
| `helmOperator.configureRepositories.volumeName`   | `repositories-yaml`                                  | Name of the volume for the `repositories.yaml` file
| `helmOperator.configureRepositories.secretName`   | `flux-helm-repositories`                             | Name of the secret containing the contents of the `repositories.yaml` file
| `helmOperator.configureRepositories.cacheName`    | `repositories-cache`                                 | Name for the repository cache volume
| `helmOperator.configureRepositories.repositories` | `None`                                               | List of custom Helm repositories to add. If non empty, the corresponding secret with a `repositories.yaml` will be created
| `helmOperator.resources.requests.cpu`             | `50m`                                                | CPU resource requests for the helmOperator deployment
| `helmOperator.resources.requests.memory`          | `64Mi`                                               | Memory resource requests for the helmOperator deployment
| `helmOperator.resources.limits`                   | `None`                                               | CPU/memory resource limits for the helmOperator deployment
| `helmOperator.nodeSelector`                       | `{}`                                                 | Node Selector properties for the helmOperator deployment
| `helmOperator.tolerations`                        | `[]`                                                 | Tolerations properties for the helmOperator deployment
| `helmOperator.affinity`                           | `{}`                                                 | Affinity properties for the helmOperator deployment
| `kube.config`                                     | [See values.yaml](/chart/flux/values.yaml#L151-L165) | Override for kubectl default config in the Flux pod(s).
| `prometheus.enabled`                              | `false`                                              | If enabled, adds prometheus annotations to Flux and helmOperator pod(s)

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example:

```sh
$ helm upgrade --install --wait flux \
--set git.url=git@github.com:stefanprodan/k8s-podinfo \
--set git.path="deploy/auto-scaling\,deploy/local-storage" \
--namespace flux \
weaveworks/flux
```

### Upgrade

Update Weave Flux version with:

```sh
helm upgrade --reuse-values flux \
--set image.tag=1.8.1 \
weaveworks/flux
```
