---
title: Using Weave Flux
menu_order: 40
---

All of the features of Flux are accessible from within
[Weave Cloud](https://cloud.weave.works).

However, `fluxctl` provides an equivalent API that can be used from
the command line.

Download the latest version of the fluxctl client
[from github](https://github.com/weaveworks/flux/releases).

The `--help` for `fluxctl` is described below.

# Connecting fluxctl to the daemon

By default, fluxctl will attempt to port-forward to your Flux
instance, assuming it runs in the `"default"` namespace. You can
specify a different namespace with the `--k8s-fwd-ns` flag:

```
fluxctl --k8s-fwd-ns=weave list-controllers
```

The namespace can also be given in the environment variable
`FLUX_FORWARD_NAMESPACE`:

```
export FLUX_FORWARD_NAMESPACE=weave
fluxctl list-controllers
```

If you are not able to use the port forward to connect, you will need
some way of connecting to the Flux API directly (NodePort,
LoadBalancer, VPN, etc). **Be aware that exposing the Flux API in this
way is a security hole, because it can be accessed without
authentication.**

Once that is set up, you can specify an API URL with `--url` or the
environment variable `FLUX_URL`:

```
fluxctl --url http://127.0.0.1:3030/api/flux list-controllers
```

## Flux API service

Now you can easily query the Flux API:

```sh
fluxctl list-controllers --all-namespaces
```

## Add an SSH deploy key to the repository

Flux connects to the repository using an SSH key. You have two
options:

### 1. Allow flux to generate a key for you.

If you don't specify a key to use, Flux will create one for you. Obtain
the public key through fluxctl:

```sh
$ fluxctl identity
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDCN2ECqUFMR413CURbLBcG41fLY75SfVZCd3LCsJBClVlEcMk4lwXxA3X4jowpv2v4Jw2qqiWKJepBf2UweBLmbWYicHc6yboj5o297//+ov0qGt/uRuexMN7WUx6c93VFGV7Pjd60Yilb6GSF8B39iEVq7GQUC1OZRgQnKZWLSQ==
0c:de:7d:47:52:cf:87:61:52:db:e3:b8:d8:1a:b5:ac
+---[RSA 1024]----+
|            ..=  |
|             + B |
|      .     . +.=|
|     . + .   oo o|
|      . S . .o.. |
|           .=.o  |
|           o =   |
|            +    |
|           E     |
+------[MD5]------+
```

Alternatively, you can see the public key in the `flux` log.

The public key will need to be given to the service hosting the Git
repository. For example, in GitHub you would create an SSH deploy key
in the repository, supplying that public key.

The `flux` logs should show that it has now connected to the
repository and synchronised the cluster.

When using Kubernetes, this key is stored as a Kubernetes secret. You
can restart `flux` and it will continue to use the same key.

### 2. Specify a key to use

Create a Kubernetes Secret from a private key:

```sh
kubectl create secret generic flux-git-deploy --from-file /path/to/identity
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

Workflow:
  fluxctl list-controllers                                           # Which controllers are running?
  fluxctl list-images --controller=deployment/foo                    # Which images are running/available?
  fluxctl release --controller=deployment/foo --update-image=bar:v2  # Release new version.

Usage:
  fluxctl [command]

Available Commands:
  automate         Turn on automatic deployment for a controller.
  deautomate       Turn off automatic deployment for a controller.
  help             Help about any command
  identity         Display SSH public key
  list-controllers List controllers currently running on the platform.
  list-images      Show the deployed and available images for a controller.
  lock             Lock a controller, so it cannot be deployed.
  policy           Manage policies for a controller.
  release          Release a new version of a controller.
  save             save controller definitions to local files in platform-native format
  unlock           Unlock a controller, so it can be deployed.
  version          Output the version of fluxctl

Flags:
  -h, --help           help for fluxctl
  -t, --token string   Weave Cloud controller token; you can also set the environment variable WEAVE_CLOUD_TOKEN or FLUX_SERVICE_TOKEN
  -u, --url string     base URL of the flux controller; you can also set the environment variable FLUX_URL (default "https://cloud.weave.works/api/flux")

Use "fluxctl [command] --help" for more information about a command.
```

# What is a Controller?

This term refers to any cluster resource responsible for the creation of
containers from versioned images - in Kubernetes these are workloads such as
Deployments, DaemonSets, StatefulSets and CronJobs.

# Viewing Controllers

The first thing to do is to check whether Flux can see any running
controllers. To do this, use the `list-controllers` subcommand:

```sh
$ fluxctl list-controllers
CONTROLLER                     CONTAINER   IMAGE                                         RELEASE  POLICY
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld:master-a000001  ready
                               sidecar     quay.io/weaveworks/sidecar:master-a000002
```

Note that the actual images running will depend on your cluster.

# Inspecting the Version of a Container

Once we have a list of controllers, we can begin to inspect which versions
of the image are running.

```sh
$ fluxctl list-images --controller default:deployment/helloworld
CONTROLLER                     CONTAINER   IMAGE                          CREATED
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

# Releasing a Controller

We can now go ahead and update a controller with the `release` subcommand.
This will check whether each controller needs to be updated, and if so,
write the new configuration to the repository.

```sh
$ fluxctl release --controller=default:deployment/helloworld --user=phil --message="New version" --update-all-images
Submitting release ...
Commit pushed: 7dc025c
Applied 7dc025c61fdbbfc2c32f792ad61e6ff52cf0590a
CONTROLLER                     STATUS   UPDATES
default:deployment/helloworld  success  helloworld: quay.io/weaveworks/helloworld:master-a000001 -> master-9a16ff945b9e

$ fluxctl list-images --controller default:deployment/helloworld
CONTROLLER                     CONTAINER   IMAGE                          CREATED
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld
                                           '-> master-9a16ff945b9e        20 Jul 16 13:19 UTC
                                               master-b31c617a0fe3        20 Jul 16 13:19 UTC
                                               master-a000002             12 Jul 16 17:17 UTC
                                               master-a000001             12 Jul 16 17:16 UTC
                               sidecar     quay.io/weaveworks/sidecar
                                           '-> master-a000002             23 Aug 16 10:05 UTC
                                               master-a000001             23 Aug 16 09:53 UTC
```

# Turning on Automation

Automation can be easily controlled from within
[Weave Cloud](https://cloud.weave.works) by selecting the "Automate"
button when inspecting a controller. But we can also do this from `fluxctl`
with the `automate` subcommand.

```sh
$ fluxctl automate --controller=default:deployment/helloworld
Commit pushed: af4bf73
CONTROLLER                     STATUS   UPDATES
default:deployment/helloworld  success

$ fluxctl list-controllers --namespace=default
CONTROLLER                     CONTAINER   IMAGE                                             RELEASE  POLICY
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld:master-9a16ff945b9e ready    automated
                               sidecar     quay.io/weaveworks/sidecar:master-a000002
```

We can see that the `list-controllers` subcommand reports that the
helloworld application is automated. Flux will now automatically
deploy a new version of a controller whenever one is available and commit
the new configuration to the version control system.

# Turning off Automation

Turning off automation is performed with the `deautomate` command:

```sh
$ fluxctl deautomate --controller=default:deployment/helloworld
Commit pushed: a54ef2c
CONTROLLER                     STATUS   UPDATES
default:deployment/helloworld  success

$ fluxctl list-controllers --namespace=default
CONTROLLER                     CONTAINER   IMAGE                                             RELEASE  POLICY
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld:master-9a16ff945b9e ready
                               sidecar     quay.io/weaveworks/sidecar:master-a000002
```

We can see that the controller is no longer automated.

# Rolling back a Controller

Rolling back can be achieved by combining:

- [`deautomate`](#turning-off-automation) to prevent Flux from automatically updating to newer versions, and
- [`release`](#releasing-a-controller) to deploy the version you want to roll back to.

```sh
$ fluxctl list-images --controller default:deployment/helloworld
CONTROLLER                     CONTAINER   IMAGE                          CREATED
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld
                                           '-> master-9a16ff945b9e        20 Jul 16 13:19 UTC
                                               master-b31c617a0fe3        20 Jul 16 13:19 UTC
                                               master-a000002             12 Jul 16 17:17 UTC
                                               master-a000001             12 Jul 16 17:16 UTC
                               sidecar     quay.io/weaveworks/sidecar
                                           '-> master-a000002             23 Aug 16 10:05 UTC
                                               master-a000001             23 Aug 16 09:53 UTC

$ fluxctl deautomate --controller=default:deployment/helloworld
Commit pushed: c07f317
CONTROLLER                     STATUS   UPDATES
default:deployment/helloworld  success

$ fluxctl release --controller=default:deployment/helloworld --update-image=quay.io/weaveworks/helloworld:master-a000001
Submitting release ...
Commit pushed: 33ce4e3
Applied 33ce4e38048f4b787c583e64505485a13c8a7836
CONTROLLER                     STATUS   UPDATES
default:deployment/helloworld  success  helloworld: quay.io/weaveworks/helloworld:master-9a16ff945b9e -> master-a000001

$ fluxctl list-images --controller default:deployment/helloworld
CONTROLLER                     CONTAINER   IMAGE                          CREATED
default:deployment/helloworld  helloworld  quay.io/weaveworks/helloworld
                                           |   master-9a16ff945b9e        20 Jul 16 13:19 UTC
                                           |   master-b31c617a0fe3        20 Jul 16 13:19 UTC
                                           |   master-a000002             12 Jul 16 17:17 UTC
                                           '-> master-a000001             12 Jul 16 17:16 UTC
                               sidecar     quay.io/weaveworks/sidecar
                                           '-> master-a000002             23 Aug 16 10:05 UTC
                                               master-a000001             23 Aug 16 09:53 UTC
```

# Locking a Controller

Locking a controller will stop manual or automated releases to that
controller. Changes made in the file will still be synced.

```sh
$ fluxctl lock --controller=deployment/helloworld
Commit pushed: d726722
CONTROLLER                     STATUS   UPDATES
default:deployment/helloworld  success
```

# Releasing an image to a locked controller

It may be desirable to release an image to a locked controller while
maintaining the lock afterwards. In order to not having to modify the
lock policy (which includes author and reason), one may use `--force`:
```
fluxctl release --controller=default:deployment/helloworld --update-all-images --force
```

# Unlocking a Controller

Unlocking a controller allows it to have manual or automated releases
(again).

```sh
$ fluxctl unlock --controller=deployment/helloworld
Commit pushed: 708b63a
CONTROLLER                     STATUS   UPDATES
default:deployment/helloworld  success
```

# Recording user and message with the triggered action

Issuing a deployment change results in a version control change/git
commit, keeping the history of the actions. The Flux daemon can be
started with several flags that impact the commit information:

| flag              | purpose                       | default |
|-------------------|-------------------------------|------------|
| git-user          | committer name                | Weave Flux |
| git-email         | committer name                | support@weave.works |
| git-set-author    | override the commit author    | false |

Actions triggered by a user through the Weave Cloud UI or the CLI `fluxctl`
tool, can have the commit author information customized. This is handy for providing extra context in the
notifications and history. Whether the customization is possible, depends on the Flux daemon (fluxd)
`git-set-author` flag. If set, the commit author will be customized in the following way:

# Image Tag Filtering

When building images it is often useful to tag build images by the branch that they were built against for example:

```
quay.io/weaveworks/helloworld:master-9a16ff945b9e
```

Indicates that the "helloworld" image was built against master
commit "9a16ff945b9e".

When automation is turned on flux will, by default, use whatever
is the latest image on a given repository. If you want to only
auto-update your image against a certain subset of tags then you
can do that using tag filtering.

So for example, if you want to only update the "helloworld" image
to tags that were built against the "prod" branch then you could
do the following:

```
fluxctl policy --controller=default:deployment/helloworld --tag-all='prod-*'
```

If your pod contains multiple containers then you tag each container
individually:

```
fluxctl policy --controller=default:deployment/helloworld --tag='helloworld=prod-*' --tag='sidecar=prod-*'
``` 

Manual releases without explicit mention of the target image will
also adhere to tag filters.
This will only release the newest image matching the tag filter:
```
fluxctl release --controller=default:deployment/helloworld --update-all-images
```

To release an image outside of tag filters, either specify the image:
```
fluxctl release --controller=default:deployment/helloworld --update-image=helloworld:dev-abc123
```
or use `--force`:
```
fluxctl release --controller=default:deployment/helloworld --update-all-images --force
```

Please note that automation might immediately undo this.

## Filter pattern types

Flux currently offers support for `glob`, `semver` and `regexp` based filtering.

### Glob

The glob (`*`) filter is the simplest filter Flux supports, a filter can contain
multiple globs:
```
fluxctl policy --controller=default:deployment/helloworld --tag-all='glob:master-v1.*.*'
```

### Semver

If your images use [semantic versioning](https://semver.org) you can filter by image tags
that adhere to certain constraints:
```
fluxctl policy --controller=default:deployment/helloworld --tag-all='semver:~1'
```

or only release images that have a stable semantic version tag (X.Y.Z):
```
fluxctl policy --controller=default:deployment/helloworld --tag-all='semver:*'
```

Using a semver filter will also affect how flux sorts images, so
that the higher versions will be considered newer.

### Regexp

If your images have complex tags you can filter by regular expression:
```
fluxctl policy --controller=default:deployment/helloworld --tag-all='regexp:^([a-zA-Z]+)$'
```

Please bear in mind that if you want to match the whole tag,
you must bookend your pattern with `^` and `$`.

## Actions triggered through `fluxctl`

`fluxctl` provides the following flags for the message and author customization:

  -m, --message string      message associated with the action
      --user    string      user who triggered the action

Commit customization

    1. Commit message

       fluxctl --message="Message providing more context for the action" .....

    2. Committer

        Committer information can be overriden with the appropriate fluxd flags:

        --git-user
        --git-email

        See [site/daemon.md] for more information.

    3. Commit author

        The default for the author is the committer information, which can be overriden,
        in the following manner:

        a) Default override uses user's git configuration, ie user.name
           and user.email (.gitconfig) to set the commit author.
           If the user has neither user.name nor for
           user.email set up, the committer information will be used. If only one
           is set up, that will be used.

        b) This can be further overriden by the use of the fluxctl --user flag.

        Examples

        a) fluxctl --user="Jane Doe <jane@doe.com>" ......
            This will always succeed as git expects a new author in the format
            "some_string <some_other_string>".

        b) fluxctl --user="Jane Doe" .......
            This form will succeed if there is already a repo commit, done by
            Jane Doe.

        c) fluxctl --user="jane@doe.com" .......
            This form will succeed if there is already a repo commit, done by
            jane@doe.com.

## Errors due to author customization

In case of no prior commit by the specified author, an error will be reported
for b) and c):

git commit: fatal: --author 'unknown' is not 'Name <email>' and matches
no existing author
