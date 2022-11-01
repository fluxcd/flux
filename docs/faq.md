# Frequently asked questions

## Migrate to Flux v2

[Flux v1 is in maintenance](https://github.com/fluxcd/flux/issues/3320) on the road to becoming formally superseded by Flux v2. Flux users are all encouraged to [migrate to Flux v2](/flux/migration/flux-v1-migration/) as early as possible.

### Why should I upgrade

Flux v2 includes some breaking changes, which means there is some work required to migrate. We hope that Flux users can all be persuaded to upgrade. There are some great reasons to follow the Flux organization's latest hard work and consider upgrading to Flux v2:

#### Flux v1 runtime behavior doesn't scale well

While there are many Flux v1 users in production, and some of them are running at very large scales, Flux users with smaller operations or those that haven't needed to scale maybe didn't notice that Flux v1 actually doesn't scale very well at all.

Some architectural issues in the original design of Flux weren't practical to resolve, or weren't known, until after implementations could be attempted at scale.

One can debate the design choices made in Flux v1 vs. Flux v2, but it was judged by the maintainers that the design of Flux importantly required some breaking changes to resolve some key architectural issues.

Flux v1 implementation of image automation has serious performance issues scaling into thousands of images. This came to a head when Docker Hub started rate-limiting image pulls, because of the expense of this operation performed casually and at scale.

That's right, rate limiting undoutedly happened because of abusive clients pulling image metadata from many images (like Flux v1 did,) images that might only be stored for the purpose of retention policies, that might be relegated to cold storage if they were not being periodically retrieved.

Flux v2 resolved this with [sortable image tags](/flux/guides/sortable-image-tags/); (this is a breaking change.)

Flux v1 requires one Flux daemon to be running per git repository/branch that syncs to the cluster. Flux v2 only expects cluster operators to run one source-controller instance, allowing to manage multiple repositories, or multiple clusters (or an entire fleet) with just one Flux installation.

Fundamentally, Flux v1 was one single configuration and reconciliation per daemon process, while Flux v2 is designed to handle many configurations for concurrent resources like git repositories, helm charts, helm releases, tenants, clusters, Kustomizations, git providers, alert providers, (... the list continues to grow.)

#### Flux v2 is more reliable and observable

As many advanced Flux v1 users will know, Flux's manifest generation capabilities come at a heavy cost. If manifest generation takes too long, timeout errors in Flux v1 can pre-empt the main loop and prevent the reconciliation of manifests altogether. The effect of this circumstance is a complete Denial-of-Service for changes to any resources managed by Flux v1 — it goes without saying, this is very bad.

Failures in Flux v2 are handled gracefully, with each controller performing separate reconciliations on the resources in their domain. One Kustomization can fail reconciling, or one GitRepository can fail syncing (for whatever reason including its own configurable timeout period), without interrupting the whole Flux system.

An error is captured as a Kubernetes `Event` CRD, and is reflected in the `Status` of the resource that had an error. When there is a fault, the new design allows that other processes should not be impacted by the fault.

#### Flux v2 covers new use cases

There is an idealized use case of GitOps we might explain as: when an update comes, a pull-request is automatically opened and when it gets merged, it is automatically applied to the cluster. That sounds great, but is not really how things work in Flux v1.

In Flux v2, this can actually be used as a real strategy; it is straight-forward to implement and covered by documenation: [Push updates to a different branch](/flux/guides/image-update/#push-updates-to-a-different-branch).

In Flux v1, it was possible to set up incoming webhooks with [flux-recv](https://github.com/fluxcd/flux-recv) as a sidecar to Flux, which while it worked nicely, it isn't nicely integrated and frankly feels bolted-on, sort of like an after-market part. This may be more than appearance, it isn't mentioned at all in Flux v1 docs!

The notification-controller is a core component in the architecture of Flux v2 and the `Receiver` CRD can be used to configure similar functionality with included support for the multi-repository features of Flux.

Similarly, in Flux v1 it was possible to send notifications to outside webhooks like Slack, MS Teams, and GitHub, but only with the help of third-party tools like [justinbarrick/fluxcloud](https://github.com/justinbarrick/fluxcloud). This functionality has also been subsumed as part of notification-controller and the `Alert` CRD can be used to configure outgoing notifications for a growing list of alerting providers today!

#### Flux v2 takes advantage of Kubernetes Extension API

The addition of CRDs to the design of Flux is another great reason to upgrade. Flux v1 had a very limited API which was served from the Flux daemon, usually controlled by using `fluxctl`, which has limited capabilities of inspection, and limited control over the behavior. By using CRDs, Flux v2 can take advantage of the Kubernetes API's extensibility so Flux itself doesn't need to run any daemon which responds directly to API requests.

Operations through Custom Resources (CRDs) provide great new opportunities for observability and eventing as was explained already, and also provides greater reliability through centralization.

Using one centralized, highly-available API service (the Kubernetes API) not only improves reliability, but is a great move for security as well; this decision reduces the risk that when new components are added, growing the functionality of the API, with each step we take we are potentially growing the attack surfaces.

The Kubernetes API is secured by default with TLS certificates for authentication and mandates RBAC configuration for authorization. It's also available in every namespace on the cluster, with a default service account. This is a highly secure API design, and it is plain to see this implementation has many eyes on it.

#### Flux v1 won't be supported forever

The developers of Flux have committed to maintain Flux v1 to support their production user-base for a reasonable span of time.

It's understood that many companies cannot adopt Flux v2 while it remains in prerelease state. So [Flux v1 is in maintenance mode](https://github.com/fluxcd/flux/issues/3320), which will continue at least until a GA release of Flux v2 is announced, and security updates and critical fixes can be made available for at least 6 months following that time.

System administrators often need to plan their migrations far in advance, and these dates won't remain on the horizon forever. It was announced as early as August 2019 that Flux v2 would be backward-incompatible, and that users would eventually be required to upgrade in order to continue to receive support from the maintainers.

Many users of Flux have already migrated production environments to Flux v2. Consider that the sooner this upgrade is undertaken by your organization, the sooner you can get it over with and have put this all behind you.

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

No, Flux will not update Git based on changes to the clusters performed through some other means. It applies changes to git only when a Flux command or API call makes them. For example, when [automated image updates](references/automated-image-update.md) are enabled.

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
[SSH config](https://man7.org/linux/man-pages/man5/ssh_config.5.html). This
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
trigger a build. You can avoid this by supplying the flag `--git-ci-skip`
so that Flux's commit will append `[ci skip]` to its commit
messages. Many CI systems will treat that as meaning they should not
run a build for that commit. You can use `--git-ci-skip-message`, if you
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

Yes. The default namespace can be changed by passing the command-line flag
`--k8s-default-namespace` to `fluxd`.

### Can I temporarily make Flux ignore a manifest?

Yes. The easiest way to do that is to use the following annotation in the manifest, and commit
the change to git:

```yaml
    fluxcd.io/ignore: "true"
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
