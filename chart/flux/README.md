# Flux

Flux is a tool that automatically ensures that the state of a cluster matches the config in git.
It uses an operator in the cluster to trigger deployments inside Kubernetes, which means you don't need a separate CD tool.
It monitors all relevant image repositories, detects new images, triggers deployments and updates the desired running
configuration based on that (and a configurable policy).

## Introduction

This chart bootstraps a [Flux](https://github.com/fluxcd/flux) deployment on
a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

Kubernetes >= v1.11

# Git repo

 - One repo containing cluster config (i.e., Kubernetes YAMLs)
 - Example of a test repo: https://github.com/fluxcd/flux-get-started

## Installation

We put together a simple [Get Started
tutorial](https://docs.fluxcd.io/en/stable/tutorials/get-started-helm.html) which takes about 5-10 minutes to follow.
You will have a fully working Flux installation deploying workloads to your cluster.

## Installing Flux using Helm

The [configuration](#configuration) section lists all the parameters that can be configured during installation.

### Installing the Chart

Add the Flux repo:

```sh
helm repo add fluxcd https://charts.fluxcd.io
```

#### Install the chart with the release name `flux`

1. Create the flux namespace:

   ```sh
   kubectl create namespace flux
   ```

1. Replace `fluxcd/flux-get-started` with your own git repository and run helm install:

   ```sh
   helm upgrade -i flux fluxcd/flux \
   --set git.url=git@github.com:fluxcd/flux-get-started \
   --namespace flux
   ```

1. Setup Git deploy

   > **Note:** this not required when [using git over HTTPS](#flux-with-git-over-https).

   At startup Flux generates a SSH key and logs the public key. Find the
   SSH public key by installing [fluxctl](https://docs.fluxcd.io/en/stable/references/fluxctl.html)
   and running:

   ```sh
   fluxctl identity --k8s-fwd-ns flux
   ```

   In order to sync your cluster state with GitHub you need to copy the
   public key and create a deploy key with access on your GitHub
   repository.  Go to _Settings > Deploy keys_ click on _Add deploy key_,
   paste the Flux public key and click _Add key_. If you want Flux to
   have write access to your repo, check _Allow write access_; if you
   have set `git.readonly=true`, you can leave this box unchecked.

#### Flux with git over HTTPS

By setting the `env.secretName`, all key/value pairs in this secret will
be defined in the Flux container as environment variables. This can be
utilized in combination with Kubernetes feature of [using environment
variables inside of your config](https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-container/#using-environment-variables-inside-of-your-config)
to securely provide the HTTPS credentials which then can be used in the
`git.url`.

1. Create a personal access token to be used as the `GIT_AUTHKEY`:

   - [GitHub](https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line)
   - [GitLab](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html#creating-a-personal-access-token)
   - [BitBucket](https://confluence.atlassian.com/bitbucketserver/personal-access-tokens-939515499.html)

1. Create a secret with your `GIT_AUTHUSER` (the username the token belongs
   to) and the `GIT_AUTHKEY` you created in the first step:

   ```sh
   kubectl create secret generic flux-git-auth --namespace flux --from-literal=GIT_AUTHUSER=<username> --from-literal=GIT_AUTHKEY=<token>
   ```

1. Install Flux:

   ```sh
   helm upgrade -i flux fluxcd/flux \
   --set git.url='https://$(GIT_AUTHUSER):$(GIT_AUTHKEY)@github.com/fluxcd/flux-get-started.git' \
   --set env.secretName=flux-git-auth \
   --namespace flux
   ```

#### Flux with a private git host

When using a private git host, setting the `ssh.known_hosts` variable
is required for enabling successful key matches because `StrictHostKeyChecking`
is enabled during Flux git daemon operations.

By setting the `ssh.known_hosts` variable, a configmap will be created
called `flux-ssh-config` which in turn will be mounted into a volume named
`sshdir` at `/root/.ssh/known_hosts`.

1. Get the `ssh.known_hosts` keys by running the following command:

   ```sh
   ssh-keyscan <your_git_host_domain>
   ```

   To prevent a potential man-in-the-middle attack, one should
   verify the ssh keys acquired through the `ssh-keyscan` match expectations
   using an alternate mechanism.

1. Install Flux:

   - Using a string for setting `known_hosts`

     ```sh
     YOUR_GIT_HOST=your_git_host.example.com
     YOUR_GIT_USER=your_git_user
     KNOWN_HOSTS='domain ssh-rsa line1
     domain ecdsa-sha2-line2
     domain ssh-ed25519 line3'

     helm upgrade -i flux fluxcd/flux \
     --set git.url="git@${YOUR_GIT_HOST}:${YOUR_GIT_USER}/flux-get-started" \
     --set-string ssh.known_hosts="${KNOWN_HOSTS}" \
     --namespace flux
     ```

   - Using a file for setting `known_hosts`

     Copy `known_hosts` keys into a temporary file `/tmp/flux_known_hosts`

     ```sh
     YOUR_GIT_HOST=your_git_host.example.com
     YOUR_GIT_USER=your_git_user

     helm upgrade -i flux fluxcd/flux \
     --set git.url="git@${YOUR_GIT_HOST}:${YOUR_GIT_USER}/flux-get-started" \
     --set-file ssh.known_hosts=/tmp/flux_known_hosts \
     --namespace flux
     ```

#### Connect Flux to a Weave Cloud instance

```sh
helm upgrade -i flux fluxcd/flux \
--set git.url=git@github.com:fluxcd/flux-get-started \
--set token=YOUR_WEAVE_CLOUD_SERVICE_TOKEN \
--namespace flux
```

### Uninstalling the Chart

To uninstall/delete the `flux` deployment:

```sh
helm delete flux
```

The command removes all the Kubernetes components associated with the chart and deletes the release.
You should also remove the deploy key from your GitHub repository.

### Configuration

The following tables lists the configurable parameters of the Flux chart and their default values.

| Parameter                                         | Default                                              | Description
| -----------------------------------------------   | ---------------------------------------------------- | ---
| `image.repository`                                | `docker.io/fluxcd/flux`                              | Image repository
| `image.tag`                                       | `<VERSION>`                                          | Image tag
| `replicaCount`                                    | `1`                                                  | Number of Flux pods to deploy, more than one is not desirable.
| `image.pullPolicy`                                | `IfNotPresent`                                       | Image pull policy
| `image.pullSecret`                                | `None`                                               | Image pull secret
| `logFormat`                                       | `fmt`                                                | Log format (fmt or json)
| `resources.requests.cpu`                          | `50m`                                                | CPU resource requests for the Flux deployment
| `resources.requests.memory`                       | `64Mi`                                               | Memory resource requests for the Flux deployment
| `resources.limits`                                | `None`                                               | CPU/memory resource limits for the Flux deployment
| `nodeSelector`                                    | `{}`                                                 | Node Selector properties for the Flux deployment
| `tolerations`                                     | `[]`                                                 | Tolerations properties for the Flux deployment
| `affinity`                                        | `{}`                                                 | Affinity properties for the Flux deployment
| `extraVolumeMounts`                               | `[]`                                                 | Extra volumes mounts
| `extraVolumes`                                    | `[]`                                                 | Extra volumes
| `dnsPolicy`                                       | ``                                                   | Pod DNS policy
| `dnsConfig`                                       | ``                                                   | Pod DNS config
| `token`                                           | `None`                                               | Weave Cloud service token
| `extraEnvs`                                       | `[]`                                                 | Extra environment variables for the Flux pod(s)
| `env.secretName`                                  | ``                                                   | Name of the secret that contains environment variables which should be defined in the Flux container (using `envFrom`)
| `rbac.create`                                     | `true`                                               | If `true`, create and use RBAC resources
| `rbac.pspEnabled`                                 | `false`                                              | If `true`, create and use a restricted pod security policy for Flux pod(s)
| `allowedNamespaces`                               | `[]`                                                 | Allow flux to manage resources in the specified namespaces. The namespace flux is deployed in will always be included
| `serviceAccount.create`                           | `true`                                               | If `true`, create a new service account
| `serviceAccount.name`                             | `flux`                                               | Service account to be used
| `serviceAccount.annotations`                      | ``                                                   | Additional Service Account annotations
| `clusterRole.create`                              | `true`                                               | If `false`, Flux will be restricted to the namespaces given in `allowedNamespaces` and the namespace where it is deployed
| `service.type`                                    | `ClusterIP`                                          | Service type to be used (exposing the Flux API outside of the cluster is not advised)
| `service.port`                                    | `3030`                                               | Service port to be used
| `sync.state`                                      | `git`                                                | Where to keep sync state; either a tag in the upstream repo (`git`), or as an annotation on the SSH secret (`secret`)
| `sync.timeout`                                    | `None`                                               | Duration after which sync operations time out (defaults to `1m`)
| `sync.interval`                                   | `<git.pollInterval>`                                 | Controls how often Flux will apply what’s in git, to the cluster, absent new commits (defaults to `git.pollInterval`)
| `git.url`                                         | `None`                                               | URL of git repo with Kubernetes manifests
| `git.readonly`                                    | `false`                                              | If `true`, the git repo will be considered read-only, Flux will not attempt to write to it
| `git.branch`                                      | `master`                                             | Branch of git repo to use for Kubernetes manifests
| `git.path`                                        | `None`                                               | Path within git repo to locate Kubernetes manifests (relative path)
| `git.user`                                        | `Weave Flux`                                         | Username to use as git committer
| `git.email`                                       | `support@weave.works`                                | Email to use as git committer
| `git.setAuthor`                                   | `false`                                              | If set, the author of git commits will reflect the user who initiated the commit and will differ from the git committer.
| `git.signingKey`                                  | `None`                                               | If set, commits will be signed with this GPG key
| `git.verifySignatures`                            | `false`                                              | If set, the signatures of the sync tag and commits will be verified
| `git.label`                                       | `flux-sync`                                          | Label to keep track of sync progress, used to tag the Git branch
| `git.ciSkip`                                      | `false`                                              | Append "[ci skip]" to commit messages so that CI will skip builds
| `git.pollInterval`                                | `5m`                                                 | Period at which to poll git repo for new commits
| `git.timeout`                                     | `20s`                                                | Duration after which git operations time out
| `git.secretName`                                  | `None`                                               | Kubernetes secret with the SSH private key. Superseded by `helmOperator.git.secretName` if set.
| `git.config.enabled`                              | `false`                                              | Mount `$HOME/.gitconfig` via Secret into the Flux and HelmOperator Pods, allowing for custom global Git configuration
| `git.config.secretName`                           | `Computed`                                           | Kubernetes secret with the global Git configuration
| `git.config.data`                                 | `None`                                               | Global Git configuration per [git-config](https://git-scm.com/docs/git-config)
| `podLabels`                                       | `{}`                                                 | Additional labels for the Flux pod
| `gpgKeys.secretName`                              | `None`                                               | Kubernetes secret with GPG keys the Flux daemon should import
| `gpgKeys.configMapName`                           | `None`                                               | Kubernetes config map with public GPG keys the Flux daemon should import
| `sops.enabled`                                    | `false`                                              | If `true` SOPS support will be enabled
| `ssh.known_hosts`                                 | `None`                                               | The contents of an SSH `known_hosts` file, if you need to supply host key(s)
| `registry.automationInterval`                     | `5m`                                                 | Period at which to check for updated images
| `registry.rps`                                    | `200`                                                | Maximum registry requests per second per host
| `registry.burst`                                  | `125`                                                | Maximum number of warmer connections to remote and memcache
| `registry.trace`                                  | `false`                                              | Output trace of image registry requests to log
| `registry.insecureHosts`                          | `None`                                               | Use HTTP rather than HTTPS for the image registry domains
| `registry.cacheExpiry`                            | `None`                                               | Duration to keep cached image info (deprecated)
| `registry.disableScanning`                        | `false`                                              | Disable registry scanning completely. Flux will be deployed without memcached
| `registry.excludeImage`                           | `None`                                               | Do not scan images that match these glob expressions; if empty, 'k8s.gcr.io/*' images are excluded
| `registry.includeImage`                           | `None`                                               | Scan only images that match these glob expressions; if empty, all images are included
| `registry.useTimestampLabels`                     | `None`                                               | Allow usage of (RFC3339) timestamp labels from (canonical) image refs that match these glob expressions; if empty, 'index.docker.io/{weaveworks,fluxcd}/*' images are allowed
| `registry.ecr.region`                             | `None`                                               | Restrict ECR scanning to these AWS regions; if empty, only the cluster's region will be scanned
| `registry.ecr.includeId`                          | `None`                                               | Restrict ECR scanning to these AWS account IDs; if empty, all account IDs that aren't excluded may be scanned
| `registry.ecr.excludeId`                          | `602401143452`                                       | Do not scan ECR for images in these AWS account IDs; the default is to exclude the EKS system account
| `registry.ecr.require`                            | `false`                                              | Refuse to start if the AWS API is not available
| `registry.acr.enabled`                            | `false`                                              | Mount `azure.json` via HostPath into the Flux Pod, enabling Flux to use AKS's service principal for ACR authentication
| `registry.acr.hostPath`                           | `/etc/kubernetes/azure.json`                         | Alternative location of `azure.json` on the host
| `registry.acr.secretName`                         | `None`                                               | Secret to mount instead of a hostPath
| `registry.dockercfg.enabled`                      | `false`                                              | Mount `config.json` via Secret into the Flux Pod, enabling Flux to use a custom docker config file
| `registry.dockercfg.secretName`                   | `None`                                               | Kubernetes secret with the docker config.json
| `registry.dockercfg.configFileName`               | `/dockercfg/config.json`                             | Alternative path/name of the docker config.json
| `memcached.enabled`                               | `true`                                               | Create a memcached deployment and service. When set to `false` you must set an external memcached service.
| `memcached.hostnameOverride`                      | `None`                                               | Override the hostname to the memcached service. Useful when using memcached deployed separately from this chart.
| `memcached.verbose`                               | `false`                                              | Enable request logging in memcached
| `memcached.maxItemSize`                           | `5m`                                                 | Maximum size for one item
| `memcached.maxMemory`                             | `128`                                                | Maximum memory to use, in megabytes
| `memcached.pullSecret`                            | `None`                                               | Image pull secret
| `memcached.repository`                            | `memcached`                                          | Image repository
| `memcached.resources`                             | `None`                                               | CPU/memory resource requests/limits for memcached
| `memcached.securityContext`                       | [See values.yaml](/chart/flux/values.yaml#L176-L179) | Container security context for memcached
| `memcached.nodeSelector`                          | `{}`                                                 | Node Selector properties for the memcached deployment
| `memcached.tolerations`                           | `[]`                                                 | Tolerations properties for the memcached deployment
| `kube.config`                                     | [See values.yaml](/chart/flux/values.yaml#L200-L212) | Override for kubectl default config in the Flux pod(s).
| `prometheus.enabled`                              | `false`                                              | If enabled, adds prometheus annotations to Flux and helmOperator pod(s)
| `prometheus.serviceMonitor.create`                | `false`                                              | Set to true if using the Prometheus Operator
| `prometheus.serviceMonitor.interval`              | ``                                                   | Interval at which metrics should be scraped
| `prometheus.serviceMonitor.namespace`             | ``                                                   | The namespace where the ServiceMonitor is deployed
| `prometheus.serviceMonitor.additionalLabels`      | `{}`                                                 | Additional labels to add to the ServiceMonitor
| `syncGarbageCollection.enabled`                   | `false`                                              | If enabled, fluxd will delete resources that it created, but are no longer present in git (see [garbage collection](/docs/references/garbagecollection.md))
| `syncGarbageCollection.dry`                       | `false`                                              | If enabled, fluxd won't delete any resources, but log the garbage collection output (see [garbage collection](/docs/references/garbagecollection.md))
| `manifestGeneration`                              | `false`                                              | If enabled, fluxd will look for `.flux.yaml` and run Kustomize or other manifest generators
| `hostAliases`                                     | `{}`                                                 | Additional hostAliases to add to the Flux pod(s). See <https://kubernetes.io/docs/concepts/services-networking/add-entries-to-pod-etc-hosts-with-host-aliases/>

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example:

```sh
helm upgrade -i flux fluxcd/flux \
--set git.url=git@github.com:stefanprodan/k8s-podinfo \
--set git.path="deploy/auto-scaling\,deploy/local-storage" \
--namespace flux \
fluxcd/flux
```

### Upgrade

Update Flux version with:

```sh
helm upgrade --reuse-values flux fluxcd/flux \
--set image.tag=1.17.1
```
