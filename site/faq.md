---
title: Flux FAQ
menu_order: 60
---

- [General questions](#general-questions)
  * [What does Flux do?](#what-does-flux-do)
  * [How does it automate deployment?](#how-does-it-automate-deployment)
  * [How is that different from a bash script?](#how-is-that-different-from-a-bash-script)
  * [Why should I automate deployment?](#why-should-i-automate-deployment)
  * [I thought Flux was about service routing?](#i-thought-flux-was-about-service-routing)
  * [Are there prerelease builds I can run?](#are-there-prerelease-builds-i-can-run)
- [Technical questions](#technical-questions)
  * [Does it work only with one git repository?](#does-it-work-only-with-one-git-repository)
  * [Do I have to put my application code and config in the same git repo?](#do-i-have-to-put-my-application-code-and-config-in-the-same-git-repo)
  * [Is there any special directory layout I need in my git repo?](#is-there-any-special-directory-layout-i-need-in-my-git-repo)
  * [Why does Flux need a git ssh key with write access?](#why-does-flux-need-a-git-ssh-key-with-write-access)
  * [Does Flux automatically sync changes back to git?](#does-flux-automatically-sync-changes-back-to-git)
  * [Will Flux delete resources when I remove them from git?](#will-flux-delete-resources-when-i-remove-them-from-git)
  * [How do I give Flux access to an image registry?](#how-do-i-give-flux-access-to-an-image-registry)
  * [How often does Flux check for new images?](#how-often-does-flux-check-for-new-images)
  * [How often does Flux check for new git commits (and can I make it sync faster)?](#how-often-does-flux-check-for-new-git-commits-and-can-i-make-it-sync-faster)
  * [How do I use my own deploy key?](#how-do-i-use-my-own-deploy-key)
  * [How do I use a private git host (or one that's not github.com, gitlab.com, bitbucket.org, dev.azure.com, or vs-ssh.visualstudio.com)?](#how-do-i-use-a-private-git-host-or-one-thats-not-githubcom-gitlabcom-bitbucketorg-devazurecom-or-vs-sshvisualstudiocom)
  * [Why does my CI pipeline keep getting triggered?](#why-does-my-ci-pipeline-keep-getting-triggered)
  * [Can I restrict the namespaces that Flux can see or operate on?](#can-i-restrict-the-namespaces-that-flux-can-see-or-operate-on)
  * [Can I change the namespace Flux puts things in by default?](#can-i-change-the-namespace-flux-puts-things-in-by-default)
  * [Can I temporarily make Flux ignore a deployment?](#can-i-temporarily-make-flux-ignore-a-deployment)
  * [How can I prevent Flux overriding the replicas when using HPA?](#how-can-i-prevent-flux-overriding-the-replicas-when-using-hpa)
  * [Can I disable Flux registry scanning?](#can-i-disable-flux-registry-scanning)
  * [Does Flux support Kustomize/My favorite manifest factorization technology?](#does-flux-support-kustomizetemplatingmy-favorite-manifest-factorization-technology)
- [Flux Helm Operator questions](#flux-helm-operator-questions)
  * [I'm using SSL between Helm and Tiller. How can I configure Flux to use the certificate?](#im-using-ssl-between-helm-and-tiller-how-can-i-configure-flux-to-use-the-certificate)
  * [I've deleted a HelmRelease file from Git. Why is the Helm release still running on my cluster?](#ive-deleted-a-helmrelease-file-from-git-why-is-the-helm-release-still-running-on-my-cluster)
  * [I've manually deleted a Helm release. Why is Flux not able to restore it?](#ive-manually-deleted-a-helm-release-why-is-flux-not-able-to-restore-it)
  * [I have a dedicated Kubernetes cluster per environment and I want to use the same Git repo for all. How can I do that?](#i-have-a-dedicated-kubernetes-cluster-per-environment-and-i-want-to-use-the-same-git-repo-for-all-how-can-i-do-that)

## General questions

Also see

- [the introduction](/site/introduction.md) for Flux's design principles
- [the troubleshooting section](/site/troubleshooting.md)

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
[weaveworks/flux-prerelease](https://hub.docker.com/r/weaveworks/flux-prerelease/tags)
and
[weaveworks/helm-operator-prerelease](https://hub.docker.com/r/weaveworks/helm-operator-prerelease/tags).

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

For more information about Flux commands see [the fluxctl docs](./fluxctl.md).

### Does Flux automatically sync changes back to git?

No. It applies changes to git only when a Flux command or API call makes them.

### Will Flux delete resources when I remove them from git?

Flux has an experimental (for now) garbage collection feature,
enabled by passing the command-line flag `--sync-garbage-collection`
to fluxd.

The garbage collection is conservative: it is designed to not delete
resources that were not created by fluxd. This means it will sometimes
_not_ delete resources that _were_ created by fluxd, when
reconfigured. Read more about garbage collection
[here](./garbagecollection.md).

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
arguments reference](daemon.md#flags).

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

Then create a new secret named `flux-git-deploy`, using your private key as the content of the secret (you can generate the key with `ssh-keygen -q -N "" -f /full/path/to/private_key`):

`kubectl create secret generic flux-git-deploy --from-file=identity=/full/path/to/private_key`

Now restart fluxd to re-read the k8s secret (if it is running):

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
[standalone-setup.md](/site/standalone-setup.md#using-a-private-git-host).

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

Yes, though support for this is experimental at the minute.

Flux will only operate on the namespaces that its service account has
access to; so the most effective way to restrict it to certain
namespaces is to use Kubernetes' role-based access control (RBAC) to
make a service account that has restricted access itself. You may need
to experiment to find the most restrictive permissions that work for
your case.

You will need to use the (experimental) command-line flag
`--k8s-allow-namespace` to enumerate the namespaces that Flux
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
then create the configmap, in the same namespace you run Flux in, with
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

### Does Flux support Kustomize/Templating/My favorite manifest factorization technology?

Yes!

Flux experimentally supports technology-agnostic manifest factorization through
`.flux.yaml` configuration files placed in the Git repository. To enable this
feature please supply `fluxd` with flag `--manifest-generation=true`.

See [`.flux.yaml` configuration files documentation](/site/fluxyaml-config-files.md) for
further details.

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


