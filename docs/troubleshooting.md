# Troubleshooting

Also see the [issues labeled with
`FAQ`](https://github.com/fluxcd/flux/labels/FAQ), which often
explain workarounds.

## Flux is taking a long time to apply manifests when it syncs

If you notice that Flux takes tens of seconds or minutes to get
through each sync, while you can apply the same manifests very quickly
by hand, you may be running into this issue:
[fluxcd/flux#1422](https://github.com/fluxcd/flux/issues/1422).

Briefly, the problem is that mounting a volume into `$HOME/.kube`
effectively disables `kubectl`'s caching, which makes it much much
slower. You may have used such a volume mount to override
`$HOME/.kube/config`, possibly unknowingly -- the Helm chart did this
for you, prior to
[fluxcd/flux#1435](https://github.com/fluxcd/flux/pull/1435).

The remedy is to mount the override to some other place in the
filesystem, and use the environment entry `KUBECONFIG` to point
`kubectl` at it. This is what the Helm chart now does, so fixing it
may be as easy as reapplying the chart if that's what you're using.

This is also documented in the
[FAQ](./faq.md).

## `fluxctl` returns a 500 Internal Server Error

This usually indicates there's a bug in the Flux daemon somewhere -- in which case please [tell us about it](https://github.com/fluxcd/flux/issues/new)!

## Flux answers everything with `git repo is not configured`

This means Flux can't read from and write to the git repo. Check that

 - ... you've supplied a git repo URL. If it's of the form
   `https://github.com/user/repo` then you will need to use the
   SSH-style URL, `git@github.com:user/repo` instead.

 - ... the deploy key has read/write access to the repo. In
   GitHub, deploy keys are installed in the settings for a
   repository. To get the deploy key Flux is using, use `fluxctl
   identity`.

 - ... that the host where your git repo lives is in
   `~/.ssh/known_hosts` in the fluxd container. We prime the container
   _image_ with host keys for `github.com`, `gitlab.com`, `bitbucket.org`, `dev.azure.com`, and `vs-ssh.visualstudio.com`, but if you're using your own git server, you'll
   need to add its host key. See ["Using a private Git host"](./guides/use-private-git-host.md).

## I'm using GCR/GKE and I keep seeing "Quota exceeded" in logs

GCP (in general) has quite conservative API rate limiting, and Flux's
default settings can bump API usage over the limits. See
[fluxcd/flux#1016](https://github.com/fluxcd/flux/issues/1016)
for advice.

## Flux doesn't seem to be able to use my imagePullSecrets

If you're using `kubectl` v1.13.x to create them, then it may be due
to [this problem](https://github.com/fluxcd/flux/issues/1596). In
short, there was a breaking change to how `kubectl` creates secrets,
that found its way into the Kubernetes 1.13.0 release. It has been
corrected in [kubectl
v1.13.2](https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG-1.13.md#changelog-since-v1131),
so using that version or newer to create secrets should fix the
problem.

## Why are my images not showing up in the list of images?

Sometimes, instead of seeing the various images and their tags, the
output of `fluxctl list-images` shows nothing. There's a number of
reasons this can happen:

 - Flux just hasn't fetched the image metadata yet. This may be the case
   if you've only just started using a particular image in a workload.
 - Flux can't get suitable credentials for the image repository. At
   present, it looks at `imagePullSecret`s attached to workloads,
   service accounts, platform-provided credentials on GCP, AWS or Azure, and
   a Docker config file if you mount one into the `fluxd` container (see
   the [command-line usage](references/daemon.md)).
 - When using images in ECR, from EC2, the `NodeInstanceRole` for the
   worker node running `fluxd` must have permissions to query the ECR
   registry (or registries) in
   question. [`eksctl`](https://github.com/weaveworks/eksctl) and
   [`kops`](https://github.com/kubernetes/kops) (with
   [`.iam.allowContainerRegistry=true`](https://github.com/kubernetes/kops/blob/master/docs/iam_roles.md#iam-roles))
   both make sure this is the case.
 - When using images from ACR in AKS, the HostPath `/etc/kubernetes/azure.json`
   should be [mounted](https://kubernetes.io/docs/concepts/storage/volumes/) into the Flux Pod.
   Set `registry.acr.enabled=True` in the [helm chart](https://github.com/fluxcd/flux/blob/master/chart/flux/README.md#)
   or alter the [Deployment](https://github.com/fluxcd/flux/blob/master/deploy/flux-deployment.yaml):
   ```yaml
    spec:
      containers:
        image: docker.io/fluxcd/flux
        ...
        volumeMounts:
        - name: acr-credentials
          mountPath: /etc/kubernetes/azure.json
          readOnly: true
      volumes:
      - name: acr-credentials
        hostPath:
          path: /etc/kubernetes/azure.json
          type: ""
   ```
   If you encounter [permission errors](https://github.com/Azure/AKS/issues/729), 
   you can alternatively create a secret `acr-credentials` based on the
   `azure.json` file and set `registry.acr.secretName=acr-credentials`.
 - Flux excludes images with no suitable manifest (linux amd64) in manifestlist
 - Flux doesn't yet understand image refs that use digests instead of
   tags; see
   [fluxcd/flux#885](https://github.com/fluxcd/flux/issues/885).

If none of these explanations seem to apply, please
[file an issue](https://github.com/fluxcd/flux/issues/new).

## Why do my image tags appear out of order?

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
[OpenContainers pre-defined annotations](https://github.com/opencontainers/image-spec/blob/master/annotations.md).

## What is the "sync tag"; or, why do I see a `flux-sync` tag in my git repo?

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

## Flux fails with an error log similar to _couldn't get resource list for example.com/version: the server is currently unable to handle the request_

This means your Kubernetes cluster fails to respond to list queries
for resources in _example.com/version_.

If the error is transient, Flux will work once the error recedes.

However, the error won't normally go away since most of the time it's caused by 
a misconfiguration of your cluster.

For instance, you can run into this problem:
  * When a
    [Kubernetes Webhook server](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
    is removed without removing its Webhook definition.
  * When a custom resource definition (CRD) is not available due to
    a `FailedDiscoveryCheck` error.
 
We recommend trying to address the root cause by fixing your cluster
configuration. In the examples above, you would need to remove the Webhook
definition or add the CRD.

However, fixing your cluster configuration may not always be possible. The
problem is common enough that Flux provides a flag called
`--k8s-unsafe-exclude-resource`. The name says it all, you should only use it
if you know what you are doing.

`--k8s-unsafe-exclude-resource` will tell Flux to avoid querying the cluster
for those resources. This in turn means that Flux won't take into account those
excluded cluster resources when syncing. This can cause excluded resources:
  * to be unexpectedly overwritten by their corresponding definition in
    Git during a sync (even if they are annotated with
    `flux.weave.works/ignore: "true"` on the cluster-side).
  * not to be garbage-collected.

The rule of thumb is that you can use `--k8s-unsafe-exclude-resource` on
resources not matching any manifests in your Git repository. 
 
