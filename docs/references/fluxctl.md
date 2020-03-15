# `fluxctl`

`fluxctl` provides an API that can be used from the command line.

The `--help` for `fluxctl` is described below.

## Installing `fluxctl`

### Mac OS

If you are using a Mac and use Homebrew, you can simply run:

```sh
brew install fluxctl
```

### Linux

#### Ubuntu (and others): snaps

[Many Linux distributions](https://docs.snapcraft.io/installing-snapd) support
snaps these days, which makes it very easy to install `fluxctl` and stay up to
date.

To install it, simply run:

```sh
sudo snap install fluxctl
```

If you would prefer to track builds from master, run

```sh
sudo snap install fluxctl --edge
```

instead.

#### Arch Linux

Install `fluxctl` via pacman:

```sh
pacman -S fluxctl
```

### Windows

#### Chocolatey

[Chocolatey](https://chocolatey.org/) is a third party package manager for Windows.

If you haven't already installed chocolatey you will need to [do this first](https://chocolatey.org/install).

fluxctl can then be installed from the [public package repository](https://chocolatey.org/packages/fluxctl):

```powershell
choco install fluxctl
```

### Binary releases

With every release of Flux, we release binaries of `fluxctl` for Mac, Linux
and Windows. Download them from the [Flux release
page](https://github.com/fluxcd/flux/releases).

## Connecting `fluxctl` to the daemon

By default, `fluxctl` will attempt to port-forward to your Flux
instance, assuming it runs in the `"default"` namespace. You can
specify a different namespace with the `--k8s-fwd-ns` flag:

```sh
fluxctl --k8s-fwd-ns=weave list-workloads
```

The namespace can also be given in the environment variable
`FLUX_FORWARD_NAMESPACE`:

```sh
export FLUX_FORWARD_NAMESPACE=weave
fluxctl list-workloads
```

If you are not able to use the port forward to connect, you will need
some way of connecting to the Flux API directly (NodePort,
LoadBalancer, VPN, etc). **Be aware that exposing the Flux API in this
way is a security hole, because it can be accessed without
authentication.**

Once that is set up, you can specify an API URL with `--url` or the
environment variable `FLUX_URL`:

```sh
fluxctl --url http://127.0.0.1:3030/api/flux list-workloads
```

### Flux API service

Now you can easily query the Flux API:

```sh
fluxctl list-workloads --all-namespaces
```

### Add an SSH deploy key to the repository

Flux connects to the repository using an SSH key. You have two
options:

#### 1. Allow Flux to generate a key for you

If you don't specify a key to use, Flux will create one for you. Obtain
the public key through `fluxctl`:

```sh
$ fluxctl identity
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDCN2ECqUFMR413CURbLBcG41fLY75SfVZCd3LCsJBClVlEcMk4lwXxA3X4jowpv2v4Jw2qqiWKJepBf2UweBLmbWYicHc6yboj5o297//+ov0qGt/uRuexMN7WUx6c93VFGV7Pjd60Yilb6GSF8B39iEVq7GQUC1OZRgQnKZWLSQ==
```

Alternatively, you can see the public key in the `flux` log.

The public key will need to be given to the service hosting the Git
repository. For example, in GitHub you would create an SSH deploy key
in the repository, supplying that public key.

The `flux` logs should show that it has now connected to the
repository and synchronised the cluster.

When using Kubernetes, this key is stored as a Kubernetes secret. You
can restart `flux` and it will continue to use the same key.

#### 2. Specify a key to use

Create a Kubernetes Secret from a private key:

```sh
kubectl create secret generic flux-git-deploy --from-file=identity=/full/path/to/private_key
```

this will result in a secret that has the structure:

```yaml
  apiVersion: v1
  data:
    identity: <base64 encoded RSA PRIVATE KEY>
  kind: Secret
  type: Opaque
  metadata:
    ...
```

Now add the secret to the `flux-deployment.yaml` manifest:

```yaml
    ...
    spec:
      volumes:
      - name: git-key
        secret:
          secretName: flux-git-deploy
```

And add a volume mount for the container:

```yaml
    ...
    spec:
      containers:
      - name: fluxd
        volumeMounts:
        - name: git-key
          mountPath: /etc/fluxd/ssh
```

You can customise the paths and names of the chosen key with the
arguments (examples with defaults): `--k8s-secret-name=flux-git-deploy`,
`--k8s-secret-volume-mount-path=/etc/fluxd/ssh` and
`--k8s-secret-data-key=identity`

Using an SSH key allows you to maintain control of the repository. You
can revoke permission for `flux` to access the repository at any time
by removing the deploy key.

```sh
fluxctl helps you deploy your code.

Connecting:

  # To a fluxd running in namespace "default" in your current kubectl context
  fluxctl list-workloads

  # To a fluxd running in namespace "weave" in your current kubectl context
  fluxctl --k8s-fwd-ns=weave list-workloads

  # To a Weave Cloud instance, with your instance token in $TOKEN
  fluxctl --token $TOKEN list-workloads

Workflow:
  fluxctl list-workloads                                                   # Which workloads are running?
  fluxctl list-images --workload=default:deployment/foo                    # Which images are running/available?
  fluxctl release --workload=default:deployment/foo --update-image=bar:v2  # Release new version.

Usage:
  fluxctl [command]

Available Commands:
  automate       Turn on automatic deployment for a workload.
  deautomate     Turn off automatic deployment for a workload.
  help           Help about any command
  identity       Display SSH public key
  install        Print and tweak Kubernetes manifests needed to install Flux in a Cluster
  list-images    Show deployed and available images.
  list-workloads List workloads currently running in the cluster.
  lock           Lock a workload, so it cannot be deployed.
  policy         Manage policies for a workload.
  release        Release a new version of a workload.
  save           save workload definitions to local files in cluster-native format
  sync           synchronize the cluster with the git repository, now
  unlock         Unlock a workload, so it can be deployed.
  version        Output the version of fluxctl

Flags:
      --context string                  The kubeconfig context to use
  -h, --help                            help for fluxctl
      --k8s-fwd-labels stringToString   Labels used to select the fluxd pod a port forward should be created for. You can also set the environment variable FLUX_FORWARD_LABELS (default [app=flux])
      --k8s-fwd-ns string               Namespace in which fluxd is running, for creating a port forward to access the API. No port forward will be created if a URL or token is given. You can also set the environment variable FLUX_FORWARD_NAMESPACE (default "default")
      --timeout duration                Global command timeout; you can also set the environment variable FLUX_TIMEOUT (default 1m0s)
  -t, --token string                    Weave Cloud authentication token; you can also set the environment variable WEAVE_CLOUD_TOKEN or FLUX_SERVICE_TOKEN
  -u, --url string                      Base URL of the Flux API (defaults to "https://cloud.weave.works/api/flux" if a token is provided); you can also set the environment variable FLUX_URL

Use "fluxctl [command] --help" for more information about a command.
```

### Using `fluxctl install`

Installs Flux into your cluster, taking as input your Git details and namespace you want to target.

Example:

```sh
fluxctl install --git-url 'git@github.com:<your username>/flux-get-started' | kubectl -f -
```

See [here](../tutorials/get-started.html#set-up-flux) for a full tutorial which makes use of `fluxctl install`.

## Workloads

### What is a Workload?

This term refers to any cluster resource responsible for the creation of
containers from versioned images - in Kubernetes these are objects such as
Deployments, DaemonSets, StatefulSets and CronJobs.

### Viewing Workloads

The first thing to do is to check whether Flux can see any running
workloads. To do this, use the `list-workloads` subcommand:

```sh
$ fluxctl list-workloads
WORKLOAD                       CONTAINER   IMAGE                                         RELEASE  POLICY
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld:master-a000001  ready
                               sidecar     quay.io/weaveworks/sidecar:master-a000002
default:deployment/busybox     busybox     busybox:1.31.1                                ready
default:deployment/nginx       nginx       nginx:stable-alpine                           ready
```

Note that the actual images running will depend on your cluster.

You can also filter workloads by container name, using the `--container|-c` option:

```sh
$ fluxctl list-workloads --container helloworld
WORKLOAD                       CONTAINER   IMAGE                                         RELEASE  POLICY
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld:master-a000001  ready
                               sidecar     quay.io/weaveworks/sidecar:master-a000002
```

### Inspecting the Version of a Container

Once we have a list of workloads, we can begin to inspect which versions
of the image are running.

```sh
$ fluxctl list-images --workload default:deployment/helloworld
WORKLOAD                       CONTAINER   IMAGE                          CREATED
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld
                                           |   master-9a16ff945b9e        20 Jul 16 13:19 UTC
                                           |   master-b31c617a0fe3        20 Jul 16 13:19 UTC
                                           |   master-a000002             12 Jul 16 17:17 UTC
                                           '-> master-a000001             12 Jul 16 17:16 UTC
                               sidecar     quay.io/weaveworks/sidecar
                                           '-> master-a000002             23 Aug 16 10:05 UTC
                                               master-a000001             23 Aug 16 09:53 UTC
```

The arrows will point to the version that is currently running
alongside a list of other versions and their timestamps.

When using `fluxctl` in scripts, you can remove the table headers with `--no-headers` for both `list-images` and `list-workloads` command to suppress the header:

```sh
$ fluxctl list-workloads --no-headers
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld:master-a000001  ready
                               sidecar     quay.io/weaveworks/sidecar:master-a000002
$ fluxctl list-images --workload default:deployment/helloworld --no-headers
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld
```

### Releasing a Workload

We can now go ahead and update a workload with the `release` subcommand.
This will check whether each workload needs to be updated, and if so,
write the new configuration to the repository.

```sh
$ fluxctl release --workload=default:deployment/helloworld --user=phil --message="New version" --update-all-images
Submitting release ...
Commit pushed: 7dc025c
Applied 7dc025c61fdbbfc2c32f792ad61e6ff52cf0590a
WORKLOAD                     STATUS   UPDATES
default:deployment/helloworld  success  helloworld: quay.io/weaveworks/helloworld:master-a000001 -> master-9a16ff945b9e

$ fluxctl list-images --workload default:deployment/helloworld
WORKLOAD                       CONTAINER   IMAGE                          CREATED
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld
                                           '-> master-9a16ff945b9e        20 Jul 16 13:19 UTC
                                               master-b31c617a0fe3        20 Jul 16 13:19 UTC
                                               master-a000002             12 Jul 16 17:17 UTC
                                               master-a000001             12 Jul 16 17:16 UTC
                               sidecar     quay.io/weaveworks/sidecar
                                           '-> master-a000002             23 Aug 16 10:05 UTC
                                               master-a000001             23 Aug 16 09:53 UTC
```

### Turning on Automation

Automation can be easily controlled from `fluxctl`
with the `automate` subcommand.

```sh
$ fluxctl automate --workload=default:deployment/helloworld
Commit pushed: af4bf73
WORKLOAD                     STATUS   UPDATES
default:deployment/helloworld  success

$ fluxctl list-workloads --namespace=default
WORKLOAD                       CONTAINER   IMAGE                                             RELEASE  POLICY
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld:master-9a16ff945b9e ready    automated
                               sidecar     quay.io/weaveworks/sidecar:master-a000002
```

Automation can also be enabled by adding the annotation
`fluxcd.io/automated: "true"` to the deployment.

We can see that the `list-workloads` subcommand reports that the
helloworld application is automated. Flux will now automatically
deploy a new version of a workload whenever one is available and commit
the new configuration to the version control system.

### Turning off Automation

Turning off automation is performed with the `deautomate` command:

```sh
$ fluxctl deautomate --workload=default:deployment/helloworld
Commit pushed: a54ef2c
WORKLOAD                     STATUS   UPDATES
default:deployment/helloworld  success

$ fluxctl list-workloads --namespace=default
WORKLOAD                       CONTAINER   IMAGE                                             RELEASE  POLICY
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld:master-9a16ff945b9e ready
                               sidecar     quay.io/weaveworks/sidecar:master-a000002
```

We can see that the workload is no longer automated.

### Rolling back a Workload

Rolling back can be achieved by combining:

- [`deautomate`](#turning-off-automation) to prevent Flux from automatically updating to newer versions, and
- [`release`](#releasing-a-workload) to deploy the version you want to roll back to.

```sh
$ fluxctl list-images --workload default:deployment/helloworld
WORKLOAD                       CONTAINER   IMAGE                          CREATED
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld
                                           '-> master-9a16ff945b9e        20 Jul 16 13:19 UTC
                                               master-b31c617a0fe3        20 Jul 16 13:19 UTC
                                               master-a000002             12 Jul 16 17:17 UTC
                                               master-a000001             12 Jul 16 17:16 UTC
                               sidecar     quay.io/weaveworks/sidecar
                                           '-> master-a000002             23 Aug 16 10:05 UTC
                                               master-a000001             23 Aug 16 09:53 UTC

$ fluxctl deautomate --workload=default:deployment/helloworld
Commit pushed: c07f317
WORKLOAD                       STATUS   UPDATES
default:deployment/helloworld  success

$ fluxctl release --workload=default:deployment/helloworld --update-image=quay.io/weaveworks/helloworld:master-a000001
Submitting release ...
Commit pushed: 33ce4e3
Applied 33ce4e38048f4b787c583e64505485a13c8a7836
WORKLOAD                     STATUS   UPDATES
default:deployment/helloworld  success  helloworld: quay.io/weaveworks/helloworld:master-9a16ff945b9e -> master-a000001

$ fluxctl list-images --workload default:deployment/helloworld
WORKLOAD                     CONTAINER   IMAGE                          CREATED
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld
                                           |   master-9a16ff945b9e        20 Jul 16 13:19 UTC
                                           |   master-b31c617a0fe3        20 Jul 16 13:19 UTC
                                           |   master-a000002             12 Jul 16 17:17 UTC
                                           '-> master-a000001             12 Jul 16 17:16 UTC
                               sidecar     quay.io/weaveworks/sidecar
                                           '-> master-a000002             23 Aug 16 10:05 UTC
                                               master-a000001             23 Aug 16 09:53 UTC
```

### Locking a Workload

Locking a workload will stop manual or automated releases to that
workload. Changes made in the file will still be synced.

```sh
$ fluxctl lock --workload=deployment/helloworld
Commit pushed: d726722
WORKLOAD                       STATUS   UPDATES
default:deployment/helloworld  success
```

### Releasing an image to a locked workload

It may be desirable to release an image to a locked workload while
maintaining the lock afterwards. In order to not having to modify the
lock policy (which includes author and reason), one may use `--force`:

```sh
fluxctl release --workload=default:deployment/helloworld --update-all-images --force
```

### Unlocking a Workload

Unlocking a workload allows it to have manual or automated releases
(again).

```sh
$ fluxctl unlock --workload=deployment/helloworld
Commit pushed: 708b63a
WORKLOAD                       STATUS   UPDATES
default:deployment/helloworld  success
```

### Recording user and message with the triggered action

Issuing a deployment change results in a version control change/git
commit, keeping the history of the actions. The Flux daemon can be
started with several flags that impact the commit information:

| flag           | purpose                    | default
| -------------- | -------------------------- | ---
| git-user       | committer name             | `Weave Flux`
| git-email      | committer email            | `support@weave.works`
| git-set-author | override the commit author | false

Actions triggered by a user through the CLI `fluxctl`
tool, can have the commit author information customized. This is handy for providing extra context in the
notifications and history. Whether the customization is possible, depends on the Flux daemon (`fluxd`)
`git-set-author` flag. If set, the commit author will be customized in the following way:

## Image Tag Filtering

When building images it is often useful to tag build images by the branch that they were built against for example:

```sh
quay.io/weaveworks/helloworld:master-9a16ff945b9e
```

Indicates that the `helloworld` image was built against master
commit `9a16ff945b9e`.

When automation is turned on Flux will, by default, use whatever
is the latest image on a given repository. If you want to only
auto-update your image against a certain subset of tags then you
can do that using tag filtering.

So for example, if you want to only update the "helloworld" image
to tags that were built against the "prod" branch then you could
do the following:

```sh
fluxctl policy --workload=default:deployment/helloworld --tag-all='prod-*'
```

If your pod contains multiple containers then you tag each container
individually:

```sh
fluxctl policy --workload=default:deployment/helloworld --tag='helloworld=prod-*' --tag='sidecar=prod-*'
```

Manual releases without explicit mention of the target image will
also adhere to tag filters.
This will only release the newest image matching the tag filter:

```sh
fluxctl release --workload=default:deployment/helloworld --update-all-images
```

To release an image outside of tag filters, either specify the image:

```sh
fluxctl release --workload=default:deployment/helloworld --update-image=helloworld:dev-abc123
```

or use `--force`:

```sh
fluxctl release --workload=default:deployment/helloworld --update-all-images --force
```

Please note that automation might immediately undo this.

### Filter pattern types

Flux currently offers support for `glob`, `semver` and `regexp` based filtering.

#### Glob

The glob (`*`) filter is the simplest filter Flux supports, a filter can contain
multiple globs:

```sh
fluxctl policy --workload=default:deployment/helloworld --tag-all='glob:master-v1.*.*'
```

#### Semver

If your images use [semantic versioning](https://semver.org) you can filter by image tags
that adhere to certain constraints:

```sh
fluxctl policy --workload=default:deployment/helloworld --tag-all='semver:~1'
```

or only release images that have a stable semantic version tag (X.Y.Z):

```sh
fluxctl policy --workload=default:deployment/helloworld --tag-all='semver:*'
```

Using a semver filter will also affect how Flux sorts images, so
that the higher versions will be considered newer.

Semver has a concept of "pre-release" versions which have an extra
label like `-beta` at the end.  If you want to include these then
write a policy with a hyphen; for example `>=1.2.3` will skip
prereleases while `>=1.2.3-0` will include prereleases.

#### Regexp

If your images have complex tags you can filter by regular expression:

```sh
fluxctl policy --workload=default:deployment/helloworld --tag-all='regexp:^([a-zA-Z]+)$'
```

Instead of `regexp` it is also possible to use its alias `regex`.
Please bear in mind that if you want to match the whole tag,
you must bookend your pattern with `^` and `$`.

### Controlling image timestamps with labels

Some image registries do not expose a reliable creation timestamp for
image tags, which could pose a problem for the automated roll-out of
images.

To overcome this problem you can define one of the supported labels in
your `Dockerfile`. Flux will prioritize labels over the timestamp it
retrieves from the registry.

#### Supported label formats

- [`org.opencontainers.image.created`](https://github.com/opencontainers/image-spec/blob/master/annotations.md#pre-defined-annotation-keys)
  date and time on which the image was built (string, date-time as defined by RFC 3339).
- [`org.label-schema.build-date`](http://label-schema.org/rc1/#build-time-labels)
  date and time on which the image was built (string, date-time as defined by RFC 3339).

## Actions triggered through `fluxctl`

`fluxctl` provides the following flags for the message and author customization:

```sh
  -m, --message string      attach a message to the update
      --user    string      override the user reported as initiating the update
```

### Commit customization

1. Commit message

   ```console
   fluxctl --message="Message providing more context for the action" .....
   ```

1. Committer

    Committer information can be overriden with the appropriate fluxd flags:

    ```console
    --git-user
    --git-email
    ```

    See [daemon.md](daemon.md) for more information.

1. Commit author

    The default for the author is the committer information, which can be overriden,
    in the following manner:

    a) Default override uses user's git configuration, ie `user.name`
        and `user.email` (.gitconfig) to set the commit author.
        If the user has neither user.name nor for
        user.email set up, the committer information will be used. If only one
        is set up, that will be used.

    b) This can be further overriden by the use of the `fluxctl --user` flag.

#### Examples

1. `fluxctl --user="Jane Doe <jane@doe.com>" ......`  
   This will always succeed as git expects a new author in the format
   "some_string <some_other_string>".

1. `fluxctl --user="Jane Doe" .......`  
   This form will succeed if there is already a repo commit, done by
   Jane Doe.

1. `fluxctl --user="jane@doe.com" .......`  
   This form will succeed if there is already a repo commit, done by
   jane@doe.com.

### Errors due to author customization

In case of no prior commit by the specified author, an error will be reported
for 2) and 3):

```sh
git commit: fatal: --author 'unknown' is not 'Name <email>' and matches
no existing author
```

## Using Annotations

Automation and image tag filtering can also be managed using annotations
(`fluxctl` is using the same mechanism).

Automation can be enabled with `fluxcd.io/automated: "true"`. Image
filtering annotations take the form
`fluxcd.io/tag.<container-name>: <filter-type>:<filter-value>` or
`filter.fluxcd.io/<container-name>: <filter-type>:<filter-value>`. Values of
`filter-type` can be [`glob`](#glob), [`semver`](#semver), and
[`regexp`](#regexp). Filter values use the same syntax as when the filter is
configured using `fluxctl`.

Here's a simple but complete deployment file with annotations:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: podinfo
  namespace: demo
  labels:
    app: podinfo
  annotations:
    fluxcd.io/automated: "true"
    fluxcd.io/tag.podinfod: semver:~1.3
spec:
  selector:
    matchLabels:
      app: podinfo
  template:
    metadata:
      labels:
        app: podinfo
    spec:
      containers:
      - name: podinfod
        image: stefanprodan/podinfo:1.3.2
        ports:
        - containerPort: 9898
          name: http
        command:
        - ./podinfo
        - --port=9898
```

Things to notice:

1. The annotations are made in `metadata.annotations`, not in `spec.template.metadata`.
2. The `fluxcd.io/tag.`... references the container name `podinfod`, this will change based on your container name. If you have multiple containers you would have multiple lines like that.
3. The value for the `fluxcd.io/tag.`... annotation should includes the filter pattern type, in this case `semver`.

Annotations can also be used to tell Flux to temporarily ignore certain manifests
using `fluxcd.io/ignore: "true"`. Read more about this in the [FAQ](../faq.md).
