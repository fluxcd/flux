# Garbage collection

> **🛑 Upgrade Advisory**
>
> This documentation is for Flux (v1) which has [reached its end-of-life in November 2022](https://fluxcd.io/blog/2022/10/september-2022-update/#flux-legacy-v1-retirement-plan).
>
> We strongly recommend you familiarise yourself with the newest Flux and [migrate as soon as possible](https://fluxcd.io/flux/migration/).
>
> For documentation regarding the latest Flux, please refer to [this section](https://fluxcd.io/flux/).

Part of syncing a cluster with a git repository is getting rid of
resources in the cluster that have been removed in the repository. You
can tell `fluxd` to do this "garbage collection" using the command-line
flag `--sync-garbage-collection`. It's important to know how it
operates and appreciate its limitations before enabling it.

## How garbage collection works

When garbage collection is enabled, syncing is done in two phases:

 1. Apply all the manifests in the git repo (as delimited by the
    branch and path arguments), and give each resource a label `fluxcd.io/sync-gc-mark`
    marking it as having been synced from this source.

 2. Ask the cluster for all the resources marked as being from this
    source, and delete those that were not applied in step 1.

In the above, "source" refers to the particular combination of git
repo URL, branch, and paths that this `fluxd` has been configured to
use, which is taken as identifying the resources under _this_
`fluxd`'s control.

We need to be careful about identifying these accurately, since
getting it wrong could mean _not_ deleting resources that should be
deleted; or (much worse), deleting resources under another
`fluxd`'s control.

The definition of "source" affects how garbage collection behaves when
you reconfigure `fluxd`. It is intended to be conservative: it ensures
that `fluxd` will not delete resources that it did not create.

## Limitations of this approach

In general, if you change an element of the source (the git repo URL,
branch, and paths), there is a possibility that resources no longer
present in the new source will be missed (i.e., not deleted) by
garbage collection and you will need to delete them by hand.

| Config change     | What happens
| ----------------- | ---
| git URL or branch | If the manifests at the new git repo are the same, they will all be relabelled, and things will proceed as usual. If they are different, the resources from the old repo will be missed by garbage collection and will need to be deleted by hand
| path added        | Existing resources will be relabelled, and new resources (from manifests in the new path) will be created. Then things will proceed as usual.
| path removed      | The resources from manifests in the removed path will be missed by garbage collection, and will need to be deleted by hand. Other resources will be treated as usual.
