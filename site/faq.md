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

This also forms a base to add features like Slack integration.

### Why should I automate deployment?

Automation is a principle of lean development. It reduces waste, to 
provide efficiency gains. It empowers employees by removing dull
tasks. It mitigates against failure by avoiding silly mistakes.

## Technical questions

### Why does Flux need a deploy key?

Flux needs a deploy key to be allowed to push to the version control
system in order to read from and update the manifests.

### How do I give Flux access to a private registry?

Provide Flux with the registry credentials. See 
[an example here](/site/using.md).

### How often does Flux check for new images?

Flux polls image registries every 5 minutes by default. You can change
this, but beware that registries may throttle and even blacklist
over-eager clients (like Flux in this scenario).

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