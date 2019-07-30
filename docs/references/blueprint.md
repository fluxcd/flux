# Blueprint

This page describes the goals of Flux, how it achieves them and
significant architectural decisions. It is intentionally high level
to prevent it from being out of date too quickly.

## Flux's goals

The overall goal of Flux is to automate the deployment of services.
A typical use case would be:

1. A developer makes changes
2. An operational cluster is now out of date and needs to be updated
3. Flux observes those changes and deploys them to the cluster
4. Flux maintains the current state of the cluster (e.g. in the event of
   failure)

Hence, the goal is to automate away the need for a developer to interact
with an orchestrator (which is a common source of accidental failure) or
with the systems that ensure that the orchestrator is in a working
state.

Flux provides a CLI ([`fluxctl`](fluxctl.md)) to perform these operations
manually. Flux is flexible enough to fit into any development process.

## Implementation overview

The following describes how Flux achieves the goals.

### Synchronisation of cluster state

The Flux team firmly believe that cluster state should be version
controlled. This allows users to record the history of the cluster,
fallback to previous versions and recreate clusters in exactly the same
state when required.

But there is also another aspect. By tightly integrating the cluster
with version control, the cluster becomes more tightly integrated with
the deployment pipeline. This means that developers no longer have to
interact directly with a cluster (with the inevitable consequences of a
"fat-finger" mistake) which makes it far more stable and ideally
immutable.

Flux achieves this by automatically synchronising the state of the
cluster to match the code representing the cluster in the repository.

This simple idea then allows for a whole range of tools that can react
to changes and simply write to a repository.

### Monitoring for new images

Flux reads a list of running containers from the user git repository.
For each image, it will query the container registry to obtain the most
recently released tag.

Flux then compares the most recent image tag with that specified in the
git repository. If they don't match, the repository is updated.

When services are in an "automated" mode, the service will periodically
check to see whether there are any new images. If there are, then they
are written to the repository.

When automation is disabled, images are not checked.

In order to access private registries, credentials may be required.

### Deployment of images

Flux will only deploy different images. It will not re-deploy images
with the same tag.

Once a list of new images have been established, it will alter the
configuration of the cluster to deploy the new images.

Images can be "locked" to a specific version. "locked" images won't be
updated by automated or manual means.
