---
title: Weave Flux FAQ
menu_order: 60
---

- [General questions](#general-questions)
  * [What does Flux do?](#what-does-flux-do)
  * [How does it automate deployment?](#how-does-it-automate-deployment)
  * [How is that different from a bash script?](#how-is-that-different-from-a-bash-script)
  * [Why should I automate deployment?](#why-should-i-automate-deployment)
  * [I thought Flux was about service routing?](#i-thought-flux-was-about-service-routing)
  * [Are there nightly builds I can run?](#are-there-nightly-builds-i-can-run)
- [Technical questions](#technical-questions)
  * [Does it work only with one git repository?](#does-it-work-only-with-one-git-repository)
  * [Do I have to put my application code and config in the same git repo?](#do-i-have-to-put-my-application-code-and-config-in-the-same-git-repo)
  * [Is there any special directory layout I need in my git repo?](#is-there-any-special-directory-layout-i-need-in-my-git-repo)
  * [Why does Flux need a git ssh key with write access?](#why-does-flux-need-a-git-ssh-key-with-write-access)
  * [Does Flux automatically sync changes back to git?](#does-flux-automatically-sync-changes-back-to-git)
  * [How do I give Flux access to an image registry?](#how-do-i-give-flux-access-to-an-image-registry)
  * [How often does Flux check for new images?](#how-often-does-flux-check-for-new-images)
  * [How often does Flux check for new git commits (and can I make it sync faster)?](#how-often-does-flux-check-for-new-git-commits-and-can-i-make-it-sync-faster)
  * [How do I use my own deploy key?](#how-do-i-use-my-own-deploy-key)
  * [Why are my images not showing up in the list of images?](#why-are-my-images-not-showing-up-in-the-list-of-images)
  * [Why do my image tags appear out of order?](#why-do-my-image-tags-appear-out-of-order)
  * [How do I use a private git host (or one that's not github.com, gitlab.com, or bitbucket.org)?](#how-do-i-use-a-private-git-host-or-one-thats-not-githubcom-gitlabcom-or-bitbucketorg)
  * [Will Flux delete resources that are no longer in the git repository?](#will-flux-delete-resources-that-are-no-longer-in-the-git-repository)
  * [Why does my CI pipeline keep getting triggered?](#why-does-my-ci-pipeline-keep-getting-triggered)
  * [What is the "sync tag"; or, why do I see a `flux-sync` tag in my git repo?](#what-is-the-sync-tag-or-why-do-i-see-a-flux-sync-tag-in-my-git-repo)
  * [Can I restrict the namespaces that Flux can see or operate on?](#can-i-restrict-the-namespaces-that-flux-can-see-or-operate-on)
  * [Can I change the namespace Flux puts things in by default?](#can-i-change-the-namespace-flux-puts-things-in-by-default)
  * [Can I temporarily make Flux ignore a deployment?](#can-i-temporarily-make-flux-ignore-a-deployment)
  * [How can I prevent Flux overriding the replicas when using HPA?](#how-can-i-prevent-flux-overriding-the-replicas-when-using-hpa)
  * [Can I disable Flux registry scanning?](#can-i-disable-flux-registry-scanning)
- [Flux Helm Operator questions](#flux-helm-operator-questions)
  * [I'm using SSL between Helm and Tiller. How can I configure Flux to use the certificate?](#im-using-ssl-between-helm-and-tiller-how-can-i-configure-flux-to-use-the-certificate)
  * [I've deleted a HelmRelease file from Git. Why is the Helm release still running on my cluster?](#ive-deleted-a-helmrelease-file-from-git-why-is-the-helm-release-still-running-on-my-cluster)
  * [I've manually deleted a Helm release. Why is Flux not able to restore it?](#ive-manually-deleted-a-helm-release-why-is-flux-not-able-to-restore-it)
  * [I have a dedicated Kubernetes cluster per environment and I want to use the same Git repo for all. How can I do that?](#i-have-a-dedicated-kubernetes-cluster-per-environment-and-i-want-to-use-the-same-git-repo-for-all-how-can-i-do-that)

## General questions

Also see [the introduction](/site/introduction.md).

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

### Are there nightly builds I can run?

There are builds from CI for each merge to master branch. See
[quay.io/weaveworks/flux](https://quay.io/repository/weaveworks/flux?tab=tags)
and
[quay.io/weaveworks/helm-operator](https://quay.io/repository/weaveworks/helm-operator?tag=latest&tab=tags).

## Technical questions

### Does it work only with one git repository?

At present, yes it works only with a single git repository containing
Kubernetes manifests. You can have as many git repositories with
application code as you like, to be clear -- see
[below](#do-i-have-to-put-my-application-code-and-config-in-the-same-git-repo).

There's no principled reason for this, it's just
a consequence of time and effort being in finite supply. If you have a
use for multiple git repo support, please comment in
https://github.com/weaveworks/flux/issues/1164.

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
and will descend into subdirectories in search of YAMLs. It avoids
directories that look like Helm charts.

If you have YAML files in the repo that _aren't_ for applying to
Kubernetes, use `--git-path` to constrain where Flux starts looking.

See also [requirements.md](./requirements.md) for a little more
explanation.

### Why does Flux need a git ssh key with write access?

There are a number of Flux commands and API calls which will update the git repo in the course of
applying the command. This is done to ensure that git remains the single source of truth.

For example, if you use the following `fluxctl` command:

    fluxctl release --controller=deployment/foo --update-image=bar:v2

The image tag will be updated in the git repository upon applying the command.

For more information about Flux commands see [the fluxctl docs](./fluxctl.md).

### Does Flux automatically sync changes back to git?

No. It applies changes to git only when a Flux command or API call makes them.

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
arguments
reference](https://github.com/weaveworks/flux/blob/master/site/daemon.md#flags).

See also
[Why are my images not showing up in the list of images?](#why-are-my-images-not-showing-up-in-the-list-of-images)

### How often does Flux check for new images?

 - Flux scans image registries for metadata as quickly as it can,
   given rate limiting; and,
 - checks if any automated workloads needs updates every five minutes,
   by default.

The latter default is quite conservative, so you can try lowering it
(it's set with the flag `--registry-poll-interval`).

Please don't _increase_ the rate limiting numbers (`--registry-rps`
and `--registry-burst`) -- it's possible to get blacklisted by image
registries if you spam them with requests.

If you are using GCP/GKE/GCR, you will likely want much lower rate
limits. Please see
[weaveworks/flux#1016](https://github.com/weaveworks/flux/issues/1016)
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

Then create a new secret named `flux-git-deploy`, using your key as the content of the secret:

`kubectl create secret generic flux-git-deploy --from-file=identity=/full/path/to/key`

Now restart fluxd to re-read the k8s secret (if it is running):

`kubectl delete $(kubectl get pod -o name -l name=flux)`

### Why are my images not showing up in the list of images?

Sometimes, instead of seeing the various images and their tags, the
output of `fluxctl list-images` (or the UI in Weave Cloud, if you're
using that) shows nothing. There's a number of reasons this can
happen:

 - Flux just hasn't fetched the image metadata yet. This may be the case
   if you've only just started using a particular image in a workload.
 - Flux can't get suitable credentials for the image repository. At
   present, it looks at `imagePullSecret`s attached to workloads,
   service accounts, platform-provided credentials on GCP or AWS, and
   a Docker config file if you mount one into the fluxd container (see
   the [command-line usage](./daemon.md)).
 - When using images in ECR, from EC2, the `NodeInstanceRole` for the
   worker node running fluxd must have permissions to query the ECR
   registry (or registries) in
   question. [`eksctl`](https://github.com/weaveworks/eksctl) and
   [`kops`](https://github.com/kubernetes/kops) (with
   [`.iam.allowContainerRegistry=true`](https://github.com/kubernetes/kops/blob/master/docs/iam_roles.md#iam-roles))
   both make sure this is the case.
 - Flux excludes images with no suitable manifest (linux amd64) in manifestlist
 - Flux doesn't yet understand image refs that use digests instead of
   tags; see
   [weaveworks/flux#885](https://github.com/weaveworks/flux/issues/885).

If none of these explanations seem to apply, please
[file an issue](https://github.com/weaveworks/flux/issues/new).

### Why do my image tags appear out of order?

You may notice that the ordering given to image tags does not always
correspond with the order in which you pushed the images. That's
because Flux sorts them by the image creation time; and, if you have
retagged an older image, the creation time won't correspond to when
you pushed the image. (Why does Flux look at the image creation time?
In general there is no way for Flux to retrieve the time at which a
tag was pushed from an image registry.)

This can happen if you explicitly tag an image that already
exists. Because of the way Docker shares image layers, it can also
happen _implicitly_ if you happen to build an image that is identical
to an existing image.

If this appears to be a problem for you, one way to ensure each image
build has its own creation time is to label it with a build time;
e.g., using
[OpenContainers pre-defined annotations](https://github.com/opencontainers/image-spec/blob/master/annotations.md#pre-defined-annotation-keys).

### How do I use a private git host (or one that's not github.com, gitlab.com, or bitbucket.org)?

As part of using git+ssh securely from the Flux daemon, we make sure
`StrictHostKeyChecking` is on in the
[SSH config](http://man7.org/linux/man-pages/man5/ssh_config.5.html). This
mitigates against man-in-the-middle attacks.

We bake host keys for `github.com`, `gitlab.com`, and `bitbucket.org`
into the image to cover some common cases. If you're using another
service, or running your own git host, you need to supply your own
host key(s).

How to do this is documented in
[standalone-setup.md](/site/standalone-setup.md#using-a-private-git-host).

### Will Flux delete resources that are no longer in the git repository?

Not at present. It's tricky to come up with a safe and unsurprising
way for this to work. There's discussion of some possibilities in
[weaveworks/flux#738](https://github.com/weaveworks/flux/issues/738).

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

### What is the "sync tag"; or, why do I see a `flux-sync` tag in my git repo?

Flux keeps track of the last commit that it's applied to the cluster,
by pushing a tag (controlled by the command-line flags
`--git-sync-tag` and `--git-label`) to the git repository. This gives
it a persistent high water mark, so even if it is restarted from
scratch, it will be able to tell where it got to.

Technically, it only needs this to be able to determine which image
releases (including automated upgrades) it has applied, and that only
matters if it has been asked to report those with the `--connect`
flag. Future versions of Flux may be more sparing in use of the sync
tag.

### Can I restrict the namespaces that Flux can see or operate on?

Yes, though support for this is experimental at the minute.

Flux will only operate on the namespaces that its service account has
access to; so the most effective way to restrict it to certain
namespaces is to use Kubernetes' role-based access control (RBAC) to
make a service account that has restricted access itself. You may need
to experiment to find the most restrictive permissions that work for
your case.

You will need to use the (experimental) command-line flag
`--k8s-namespace-whitelist` to enumerate the namespaces that Flux
attempts to scan for workloads.

### Can I change the namespace Flux puts things in by default?

Yes. The fluxd image has a "kubeconfig" file baked in, which specifies
a default namespace of `"default"`. That means any manifest not
specifying a namespace (in `.metadata.namespace`) will be given the
namespace `"default"` when applied to the cluster.

You can override this by mounting your own "kubeconfig" file into the
container from a configmap, and using the `KUBECONFIG` environment
entry to point to it. The [example
deployment](../deploy/flux-deployment.yaml) shows how to do this, in
commented out sections -- it needs extra bits of config in three
places (the `volume`, `volumeMount`, and `env` entries).

The easiest way to create a suitable "kubeconfig" will be to adapt the
[file that is baked into the image](../docker/kubeconfig). Save that
locally as `my-kubeconfig`, edit it to change the default namespace,
then create the configmap, in the same namespace you run flux in, with
something like:

    kubectl create configmap flux-kubeconfig --from-file=config=./my-kubeconfig

Be aware that the expected location (`$HOME/.kube/`) of the
`kubeconfig` file is _also_ used by `kubectl` to cache API responses,
and mounting from a configmap will make it read-only and thus
effectively disable the caching. For that reason, take care to mount
your configmap elsewhere in the filesystem, as the example shows.

### Can I temporarily make Flux ignore a deployment?

Yes. The easiest way to do that is to use the following annotation
*in the manifest files*:

```yaml
    flux.weave.works/ignore: true
```

To stop ignoring these annotated resources, you simply remove the
annotation from the manifests in git. A live example can be seen
[here](https://github.com/stefanprodan/openfaas-flux/blob/master/secrets/openfaas-token.yaml).
This will work for any type of resource.

Sometimes it might be easier to annotate a *running resource in
the cluster* as opposed to committing a change to git. Please note
that this will only work with resources of the type `namespace`
and the set of controllers in
[resourcekinds.go](https://github.com/weaveworks/flux/blob/master/cluster/kubernetes/resourcekinds.go),
namely `deployment`, `daemonset`, `cronjob`, `statefulset` and
`fluxhelmrelease`).

If the annotation is just carried in the cluster, the easiest way
to remove it is to run:

```sh
kubectl annotate <resource> "flux.weave.works/ignore"-
```

Mixing both kinds of annotations (in-git and in-cluster), can make
it a bit hard to figure out how/where to undo the change (cf
[flux#1211](https://github.com/weaveworks/flux/issues/1211)).

The full story is this: Flux looks at the files and the running
resources when deciding whether what to apply. But it gets the
running resources by exporting them from the cluster, and that
only returns the kinds of resource mentioned above. So,
annotating a running resource only works if it's one of those
kinds; putting the annotation in the file always works.

### How can I prevent Flux overriding the replicas when using HPA?

When using a horizontal pod autoscaler you have to remove the `spec.replicas` from your deployment definition.
If the replicas field is not present in Git, Flux will not override the replica count set by the HPA.

### Can I disable Flux registry scanning?

You can exclude images from being scanned by providing a list of glob expressions using the `registry-exclude-image` flag.

Exclude images from Docker Hub and Quay.io:

```
--registry-exclude-image=docker.io/*,quay.io/*
```

And the Helm install equivalent (note the `\,` separator):

```
--set registry.excludeImage="docker.io/*\,quay.io/*"
```

Exclude images containing `test` in the FQN:

```
--registry-exclude-image=*test*
```

Disable image scanning for all images:

```
--registry-exclude-image=*
```

## Flux Helm Operator questions

### I'm using SSL between Helm and Tiller. How can I configure Flux to use the certificate?

When installing Flux, you can supply the CA and client-side certificate using the `helmOperator.tls` options,
more details [here](https://github.com/weaveworks/flux/blob/master/chart/flux/README.md#installing-weave-flux-helm-operator-and-helm-with-tls-enabled).

### I've deleted a HelmRelease file from Git. Why is the Helm release still running on my cluster?

Flux doesn't delete resources, there is an [issue](https://github.com/weaveworks/flux/issues/738) opened about this topic on GitHub.
In order to delete a Helm release first remove the file from Git and afterwards run:

```yaml
kubectl delete helmrelease/my-release
```

The Flux Helm operator will receive the delete event and will purge the Helm release.

### I've manually deleted a Helm release. Why is Flux not able to restore it?

If you delete a Helm release with `helm delete my-release`, the release name can't be reused.
You need to use the `helm delete --purge` option only then Flux will be able reinstall a release.

### I have a dedicated Kubernetes cluster per environment and I want to use the same Git repo for all. How can I do that?

*Option 1*
For each cluster create a directory in your config repo.
When installing Flux Helm chart set the Git path using `--set git.path=k8s/cluster-name`
and set a unique label for each cluster `--set git.label=cluster-name`.

You can have one or more shared dirs between clusters. Assuming your shared dir is located 
at `k8s/common` set the Git path as `--set git.path="k8s/common\,k8s/cluster-name"`. 

*Option 2*
For each cluster create a Git branch in your config repo. 
When installing Flux Helm chart set the Git branch using `--set git.branch=cluster-name`
and set a unique label for each cluster `--set git.label=cluster-name`.


