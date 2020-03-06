# Frequently asked questions

## General questions

Also see

- [the introduction](./introduction.md) for Flux's design principles
- [the troubleshooting section](./troubleshooting.md)

### What does Flux do?

Flux automates the process of deploying new configuration and
container images to Kubernetes.

### How does it automate deployment?

It synchronises all manifests in a repository with a Kubernetes cluster.
It also monitors container registries for new images and updates the
manifests accordingly.

### How is that different from a bash script?

The amount of functionality contained within Flux warrants a dedicated
application/service. An equivalent script could easily get too large
to maintain and reuse.

Anyway, we've already done it for you!

### Why should I automate deployment?

Automation is a principle of lean development. It reduces waste, to
provide efficiency gains. It empowers employees by removing dull
tasks. It mitigates against failure by avoiding silly mistakes.

### I thought Flux was about service routing?

That's [where we started a while
ago](https://www.weave.works/blog/flux-service-routing/). But we
discovered that automating deployments was more urgent for our own
purposes.

Staging deployments with clever routing is useful, but it's a later
level of operational maturity.

There are some pretty good solutions for service routing:
[Envoy](https://www.envoyproxy.io/), [Istio](https://istio.io) for
example. We may return to the matter of staged deployments.

### Are there prerelease builds I can run?

There are builds from CI for each merge to master branch. See
[fluxcd/flux-prerelease](https://hub.docker.com/r/fluxcd/flux-prerelease/tags).

## Technical questions

### Does it work only with one git repository?

At present, yes it works only with a single git repository containing
Kubernetes manifests. You can have as many git repositories with
application code as you like, to be clear -- see
[below](#do-i-have-to-put-my-application-code-and-config-in-the-same-git-repo).

There's no principled reason for this, it's just
a consequence of time and effort being in finite supply. If you have a
use for multiple git repo support, please comment in
https://github.com/fluxcd/flux/issues/1164.

In the meantime, for some use cases you can run more than one Flux
daemon and point them at different repos. If you do this, consider
trimming the RBAC permissions you give each daemon's service account.

This
[Flux (daemon) operator](https://github.com/justinbarrick/flux-operator)
project may be of use for managing multiple daemons.

### Do I have to put my application code and config in the same git repo?

Nope, but they can be if you want to keep them together. Flux doesn't
need to know about your application code, since it deals with
container images (i.e., once your application code has already been
built).

### Is there any special directory layout I need in my git repo?

Nope. Flux doesn't place any significance on the directory structure,
and will descend into subdirectories in search of YAMLs. Although [kubectl works with JSON files](https://kubernetes.io/docs/concepts/configuration/overview/#using-kubectl), Flux will ignore JSON. It avoids
directories that look like Helm charts.

If you have YAML files in the repo that _aren't_ for applying to
Kubernetes, use `--git-path` to constrain where Flux starts looking.

See also [requirements.md](./requirements.md) for a little more
explanation.

### Why does Flux need a git ssh key with write access?

There are a number of Flux commands and API calls which will update the git repo in the course of
applying the command. This is done to ensure that git remains the single source of truth.

For example, if you use the following `fluxctl` command:

```sh
fluxctl release --controller=deployment/foo --update-image=bar:v2
```

The image tag will be updated in the git repository upon applying the command.

For more information about Flux commands see [the `fluxctl` docs](references/fluxctl.md).

### Can I run Flux with readonly Git access?

Yes. You can use the `--git-readonly` command line argument.  The Helm
chart exposes this as `git.readonly`.

This will prevent Flux from trying to write to your repository. You
should also provide a readonly SSH key; e.g., on GitHub, leave the
`Allow write access` box unchecked when you add the deploy key.

### Does Flux automatically sync changes back to git?

No. It applies changes to git only when a Flux command or API call makes them.

### Will Flux delete resources when I remove them from git?

Flux has an garbage collection feature, enabled by passing the command-line
flag `--sync-garbage-collection` to `fluxd`.

The garbage collection is conservative: it is designed to not delete
resources that were not created by `fluxd`. This means it will sometimes
_not_ delete resources that _were_ created by `fluxd`, when reconfigured.
Read more about garbage collection [here](references/garbagecollection.md).

### How do I give Flux access to an image registry?

Flux transparently looks at the image pull secrets that you attach to
workloads and service accounts, and thereby uses the same credentials
that Kubernetes uses for pulling each image. In general, if your pods
are running, then Kubernetes has pulled the images, and Flux should be
able to access them too.

There are exceptions:

 - One way of supplying credentials in Kubernetes is to put them on each
   node; Flux does not have access to those credentials.
 - In some environments, authorisation provided by the platform is
   used instead of image pull secrets:
    - Google Container Registry works this way; Flux will
      automatically attempt to use platform-provided credentials when
      scanning images in GCR.
    - Amazon Elastic Container Registry (ECR) has its own
      authentication using IAM. If your worker nodes can read from
      ECR, then Flux will be able to access it too.

To work around exceptional cases, you can mount a docker config into
the Flux container. See the argument `--docker-config` in [the daemon
arguments reference](references/daemon.md).

For ECR, Flux requires access to the EC2 instance metadata API to
obtain AWS credentials. Kube2iam, Kiam, and potentially other
Kuberenetes IAM utilities may block pod level access to the EC2
metadata APIs. If this is the case, Flux will be unable to poll ECR
for automated workloads.

 -  If you are using Kiam, you need to whitelist the following API routes:
      ```
      --whitelist-route-regexp=(/latest/meta-data/placement/availability-zone|/latest/dynamic/instance-identity/document)
      ```
 - If you are using kube2iam, ensure the values of --iptables and
    --in-interface are [configured correctly for your virtual network
    provider](https://github.com/jtblin/kube2iam#iptables).

See also
[Why are my images not showing up in the list of images?](#why-are-my-images-not-showing-up-in-the-list-of-images)

### How often does Flux check for new images?

 - Flux scans image registries for metadata as quickly as it can,
   given rate limiting; and,
 - checks if any automated workloads needs updates every five minutes,
   by default.

The latter default is quite conservative, so you can try lowering it
(it's set with the flag `--automation-interval`).

Please don't _increase_ the rate limiting numbers (`--registry-rps`
and `--registry-burst`) -- it's possible to get blacklisted by image
registries if you spam them with requests.

If you are using GCP/GKE/GCR, you will likely want much lower rate
limits. Please see
[fluxcd/flux#1016](https://github.com/fluxcd/flux/issues/1016)
for specific advice.

### How often does Flux check for new git commits (and can I make it sync faster)?

Short answer: every five minutes; and yes.

There are two flags that control how often Flux syncs the cluster with
git. They are

 * `--git-poll-interval`, which controls how often it looks for new
   commits

 * `--sync-interval`, which controls how often it will apply what's in
   git, to the cluster, absent new commits.

Both of these have five minutes as the default. When there are new
commits, it will run a sync then and there, so in practice syncs
happen more often than `--sync-interval`.

If you want to be more responsive to new commits, then give a shorter
duration for `--git-poll-interval`, so it will check more often.

It is less useful to shorten the duration for `--sync-interval`, since
that just controls how often it will sync _without_ there being new
commits. Reducing it below a minute or so may hinder Flux, since syncs
can take tens of seconds, leaving not much time to do other
operations.

### How do I use my own deploy key?

Flux uses a k8s secret to hold the git ssh deploy key. It is possible
to provide your own.

First delete the secret (if it exists):

`kubectl delete secret flux-git-deploy`

Then create a new secret named `flux-git-deploy`, using your private key as the content of the secret (you can generate the key with `ssh-keygen -q -N "" -f /full/path/to/private_key`):

`kubectl create secret generic flux-git-deploy --from-file=identity=/full/path/to/private_key`

Now restart `fluxd` to re-read the k8s secret (if it is running):

`kubectl delete $(kubectl get pod -o name -l name=flux)`

If you have installed flux through Helm, make sure to pass
`--set git.secretName=flux-git-deploy` when installing/upgrading the chart.

### How do I use a private git host (or one that's not github.com, gitlab.com, bitbucket.org, dev.azure.com, or vs-ssh.visualstudio.com)?

As part of using git+ssh securely from the Flux daemon, we make sure
`StrictHostKeyChecking` is on in the
[SSH config](http://man7.org/linux/man-pages/man5/ssh_config.5.html). This
mitigates against man-in-the-middle attacks.

We bake host keys for `github.com`, `gitlab.com`, `bitbucket.org`, `dev.azure.com`, and `vs-ssh.visualstudio.com`
into the image to cover some common cases. If you're using another
service, or running your own git host, you need to supply your own
host key(s).

How to do this is documented in
["Using a private Git host"](./guides/use-private-git-host.md).

### Why does my CI pipeline keep getting triggered?

There's a couple of reasons this can happen.

The first is that Flux pushes commits to your git repo, and if that
repo is configured to go through CI, usually those commits will
trigger a build. You can avoid this by supplying the flag `--ci-skip`
so that Flux's commit will append `[ci skip]` to its commit
messages. Many CI systems will treat that as meaning they should not
run a build for that commit. You can use `--ci-skip-message`, if you
need a different piece of text appended to commit messages.

The other thing that can trigger CI is that Flux pushes a tag to the
upstream git repo whenever it has applied new commits. This acts as a
"high water mark" for Flux to know which commits have already been
seen. The default name for this tag is `flux-sync`, but it can be
changed with the flags `--git-sync-tag` and `--git-label`. The
simplest way to avoid triggering builds is to exclude this tag from
builds -- how to do that will depend on how your CI system is
configured.

Here's the relevant docs for some common CI systems:

 - [CircleCI](https://circleci.com/docs/2.0/workflows/#git-tag-job-execution)
 - [TravisCI](https://docs.travis-ci.com/user/customizing-the-build#Building-Specific-Branches)
 - [GitLab](https://docs.gitlab.com/ee/ci/yaml/#only-and-except-simplified)
 - [Bitbucket Pipelines](https://confluence.atlassian.com/bitbucket/configure-bitbucket-pipelines-yml-792298910.html#Configurebitbucket-pipelines.yml-ci_defaultdefault)
 - [Azure Pipelines](https://docs.microsoft.com/en-us/azure/devops/pipelines/index?view=azure-devops)

### Can I restrict the namespaces that Flux can see or operate on?

Flux will only operate on the namespaces that its service account has
access to; so the most effective way to restrict it to certain
namespaces is to use Kubernetes' role-based access control (RBAC) to
make a service account that has restricted access itself. You may need
to experiment to find the most restrictive permissions that work for
your case.

You will need to use the command-line flag `--k8s-allow-namespace`
to enumerate the namespaces that Flux attempts to scan for workloads.

### Can I change the namespace Flux puts things in by default?

Yes. The `fluxd` image has a "kubeconfig" file baked in, which specifies
a default namespace of `"default"`. That means any manifest not
specifying a namespace (in `.metadata.namespace`) will be given the
namespace `"default"` when applied to the cluster.

You can override this by mounting your own "kubeconfig" file into the
container from a configmap, and using the `KUBECONFIG` environment
entry to point to it. The [example
deployment](https://github.com/fluxcd/flux/blob/master/deploy/flux-deployment.yaml) shows how to do this, in
commented out sections -- it needs extra bits of config in three
places (the `volume`, `volumeMount`, and `env` entries).

The easiest way to create a suitable "kubeconfig" will be to adapt the
[file that is baked into the image](https://github.com/fluxcd/flux/blob/master/docker/kubeconfig). Save that
locally as `my-kubeconfig`, edit it to change the default namespace,
then create the configmap, in the same namespace you run Flux in, with
something like:

    kubectl create configmap flux-kubeconfig --from-file=config=./my-kubeconfig

Be aware that the expected location (`$HOME/.kube/`) of the
`kubeconfig` file is _also_ used by `kubectl` to cache API responses,
and mounting from a configmap will make it read-only and thus
effectively disable the caching. For that reason, take care to mount
your configmap elsewhere in the filesystem, as the example shows.

### Can I temporarily make Flux ignore a manifest?

Yes. The easiest way to do that is to use the following annotation in the manifest, and commit
the change to git:

```yaml
    fluxcd.io/ignore: true
```

To stop ignoring these annotated resources, you simply remove the annotation from the manifests in git.

Flux will ignore any resource that has the annotation _either_ in git, or in the cluster itself;
sometimes it may be easier to annotate a *running resource in the cluster* as opposed to committing
a change to git.

Mixing both kinds of annotations (in git, and in the cluster), can make it a bit hard to figure out
how/where to undo the change (cf [flux#1211](https://github.com/fluxcd/flux/issues/1211)). If the
annotation exists in either the cluster or in git, it will be respected, so you may need to remove
it from both places.

Additionally, when garbage collection is enabled, Flux will not garbage collect resources in the cluster
with the ignore annotation if the resource is removed from git.

### Can I disable garbage collection for a specific resource?

Yes. By adding the annotation below to a resource Flux will sync updates from git, but it will not
garbage collect when the resource is removed from git.

```yaml
    fluxcd.io/ignore: sync_only
```

### How can I prevent Flux overriding the replicas when using HPA?

When using a horizontal pod autoscaler you have to remove the `spec.replicas` from your deployment definition.
If the replicas field is not present in Git, Flux will not override the replica count set by the HPA.

### Can I disable Flux registry scanning?

You can completely disable registry scanning by using the
`--registry-disable-scanning` flag. This allows deploying Flux without
 Memcached.


If you only want to scan certain images, don't set
`--registry-disable-scanning`. Instead, you can tell Flux what images
to include or exclude by supplying a list of glob expressions to the
`--registry-include-image` and `--registry-exclude-image` flags:

 * `--registry-exclude-image` takes patterns to be excluded; the
   default is to exclude the Kubernetes base images (`k8s.gcr.io/*`);
   and,
 * `--registry-include-image` takes patterns to be included; no
   patterns (the default) means "include everything". If you provide a
   pattern, _only_ images matching the pattern will be included (less
   any that are explicitly excluded).

To restrict scanning to only images from organisations `example` and `example-dev`,
you might use:

```
--registry-include-image=*/example/*,*/example-dev/*
```

To exclude images from quay.io, use:

```
--registry-exclude-image=quay.io/*
```

Here are the Helm install equivalents (note the `\,` separator):

```
--set registry.includeImage="*/example/*\,*/example-dev/*" --set registry.excludeImage="quay.io/*"
```

### Does Flux support Kustomize/Templating/My favorite manifest factorization technology?

Yes!

Flux supports technology-agnostic manifest factorization through `.flux.yaml` configuration
files placed in the Git repository. To enable it supply the command-line flag
`--manifest-generation=true` to `fluxd`.

See [`.flux.yaml` configuration files documentation](references/fluxyaml-config-files.md) for
further details.
