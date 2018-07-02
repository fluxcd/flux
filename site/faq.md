---
title: Weave Flux FAQ
menu_order: 60
---

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

## Technical questions

### Does it work only with one git repository?

At present, yes it works only with a single git repository. There's no
principled reason for this, it's just a consequence of time and effort
being in finite supply. If you have a use for multiple git repo
support, please comment in
https://github.com/weaveworks/flux/issues/1164.

In the meantime, for some use cases you can run more than one Flux
daemon and point them at different repos. If you do this, consider
trimming the RBAC permissions you give each daemon's service account.

This
[flux (daemon) operator](https://github.com/justinbarrick/flux-operator)
project may be of use for managing multiple daemons.

### Do I have to have my application code and config in the same git repo?

Nope, but they can be if you want to keep them together. Flux doesn't
need to know about your application code, since it deals with
container images (i.e., once your application code has already been
built).

### Why does Flux need a deploy key?

Flux needs a deploy key to be allowed to push to the version control
system in order to read and update the manifests.

### How do I give Flux access to a private registry?

Flux transparently looks at the image pull secret that you give for a
controller, and thereby uses the same credentials that Kubernetes uses
for pulling each image. If your pods are running, Kubernetes has
pulled the images, and Flux should be able to access them.

There are exceptions: in some environments, authorisation provided by
the platform is used instead of image pull secrets. Google Container
Registry works this way, for example (and we have introduced a special
case for it so Flux will work there too).

### How often does Flux check for new images?

Short answer: every five minutes.

You can set this to be more often, to try and be responsive in
deploying new images. Be aware that there are some other things going
on:

 * Requests to image registries are rate limited, and _discovering_
   new images has some lag in itself, which depends on how many images
   you are using in total (since we go and check them one by one as
   fast as rate limiting allows).

 * Operations on the git repo are more or less
   serialised, so while you are checking for new images to deploy, you
   are not doing something else (like syncing).

Having said that, the defaults are pretty conservative, so try it and
see. Please don't increase the rate limiting numbers (`--registry-rps`
and `--registry-burst`) -- it's possible to get blacklisted by image
registries if you spam them with requests.

### How often does Flux check for new git commits (and can I make it sync faster)?

Short answer: every five minutes; and yes.

There are two flags that control how often Flux syncs the cluster with
git. They are

 * `--git-poll-interval`, which controls how often it looks for new
   commits

 * `--sync-interval`, which controls how often it will apply what's in
   git, to the cluster, absent changes.

Both of these have five minutes as the default. If there are new
commits, then it will run a sync then and there, so in practice syncs
happen more often than `--sync-interval`.

If you want to be more responsive to new commits, then give a shorter
duration for `--git-poll-interval`, so it will check more often.

It is less useful to shorten the duration for `--sync-interval`, since
that just controls how often it will sync _without_ there being new
commits. Reducing it below a minute or so may hinder Flux, since syncs
can take tens of seconds, leaving not much time to do other
operations.

### How do I use my own deploy key?

Flux uses a k8s secret to hold the git ssh deploy key. It is possible to
provide your own.

First delete the secret (if it exists):

`kubectl delete secret flux-git-deploy`

Then create a new secret named `flux-git-deploy`, using your key as the content of the secret:

`kubectl create secret generic flux-git-deploy --from-file=identity=/full/path/to/key`

Now restart fluxd to re-read the k8s secret (if it is running):

`kubectl delete $(kubectl get pod -o name -l name=flux)`

### How do I use a private docker registry?

Create a Kubernetes Secret with your docker credentials then add the
name of this secret to your Pod manifest under the `imagePullSecrets`
setting. Flux will read this value and parse the Kubernetes secret.

For a guide showing how to do this, see the
[Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/).

### How do I use a private git host (or one that's not github.com, gitlab.com, or bitbucket.com)?

As part of using git+ssh securely from the Flux daemon, we make sure
`StrictHostKeyChecking` is on in the
[SSH config](http://man7.org/linux/man-pages/man5/ssh_config.5.html). This
mitigates against man-in-the-middle attacks.

We bake host keys for `github.com`, `gitlab.com`, and `bitbucket.com`
into the image to cover some common cases. If you're using another
service, or running your own git host, you need to supply your own
host key(s).

How to do this is documented in
[setup.md](/site/standalone/setup.md#using-a-private-git-host).

### Will Flux delete resources that are no longer in the git repository?

Not at present. It's tricky to come up with a safe and unsurprising
way for this to work. There's discussion of some possibilities in
[weaveworks/flux#738](https://github.com/weaveworks/flux/issues/738).

### Why does my CI pipeline keep getting triggered?

There's a couple of reasons this can happen.

The first is that Flux pushes commits to your git repo, and if you
that repo is configured to go through CI, usually those commits will
trigger a build. You can avoid this by supplying the flag `--ci-skip`
so that Flux's commit will append `[ci skip]` to its commit
messages. Many CI system will treat that as meaning they should not
run a build for that commit. You can use `--ci-skip-message`, if you
need a different piece of text appened to commit messages.

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

### Can I restrict the namespaces that Flux can see or operate on?

Yes. Flux will only operate on the namespaces that its service account
has access to; so the most effective way to restrict it to certain
namespaces is to use Kubernetes' role-based access control (RBAC) to
make a service account that has restricted access itself. You may need
to experiment to find the most restrictive permissions that work for
your case.
