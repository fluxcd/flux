# Factoring out an image automation service

## Summary

The image database and its registry scanning behaviour, along with the
manifest file update automation --

 - depend on each other; but,
 - are logically separate to the cluster syncing part of Flux.

This RFC proposes why and how they should be separated from the
cluster syncing part.

## Motivation

 - incrementally, people have come to regard the registry scanning and
   update automation as an optional and often unwanted extra.
 - Argo Flux will need this to be a separate component
 - Breaking compatibility means we can rethink some defaults and APIs
 - It gives an opportunity to expand the design for automating other
   kinds of thing (e.g., Helm chart versions)

## Design

### Requirements and wishlist

This is what the image automation does now:

 - looks for all the workloads that it can see in the cluster, and
   extracts the set of image repos in use, along with the credentials
   used to pull from them;
 - compiles a database with the full set of image metadata (i.e., the
   manifest for each tag, for each image repo);
 - on a schedule, look for any workloads that are marked as automated,
   and check if there is a new image tag in the image repo;
 - update those workloads and commit the change to git, along with a
   git note that is later consulted by the syncing machinery;
 - update workload files (in the same way as for automated updates)
   when requested via the API;
 - update the policy annotations in manifest files, when requested via
   the API.

The image database is also consulted for some Flux API requests (used
by Weave Cloud, fluxweb, fluxctl).

These are things that could be improved in the making of a separate
component:

 - We can make certain behaviours the default without breaking
   existing deployments: specifically,
   - don't scan things you don't care about (see
     https://github.com/fluxcd/flux/issues/2864)
   - treat the git repo as the system of record for what is automated
     or otherwise updateable, rather than the cluster.
 - memcached's eviction behaviour is used as a kind of garbage
   collection, but there are better ways to do it (%%% see discussion
   on -M vs -m).
 - it would be good to be able to share a database store -- e.g.,
   memcached instance if that's what is used -- with other instances
   of this service.
 - if not immediately, this component could also scan Helm chart repos
   and do automation of chart versions in HelmRelease manifests.
 - the automation API is a bit weird (you can only select workloads to
   update in very constrained ways), and could be rationalised
 - there's no good reason to have an unbounded queue rather than just
   using a channel (or slice, depending on the synchronisation
   properties desired) and refusing jobs when it's full;
 - the release (image update) machinery is ludicrously complex and
   crufty and really needs to be simplified.
 - the resource ID stuff is unnecessary since we're only targetting
   Kubernetes (in the distant past, the idea was to target Swarm and
   maybe others), so just adopt Kubernetes' references.

### Adaption to ArgoCD

Where does this fit in with ArgoCD? Some broad principles:

 - it should have an API accessible to the Argo UI;
 - it should not require changes to the current ArgoCD architecture
   (though it may suggest additional functionality, like receiving and
   forwarding image registry webhooks);
 - it could also be run stand alone, assuming tooling using its API
 - (also see Unresolved Questions regarding custom resources)

### The existing system

Since the name of the game is to detangle the image database and
automation from the rest of Flux, it's worth examining how they are
tangled.

There are three components that form the image database and
automation:

 - the **image database** (in pkg/registry and beneath), which answers
   queries about images by looking at memcached;
 - the **image scanning** (a.k.a., cache warmer), which scans image
   registries for metadata and puts it in the database;
 - the **automation queue**, which processes scheduled or requested
   updates to manifests in git, often relying on the image database.

#### Initialisation

Most of the machinery is constructed in cmd/fluxd/main.go, so that's a
good place to look for the dependencies and other entanglements.

The **image scanning** component depends on an implementation of an
image fetch function to get the images to scan:

    func () registry.ImageCreds

which it will get eventually from the cluster component, initialised
here:
https://github.com/fluxcd/flux/blob/v1.18.0/cmd/fluxd/main.go#L412

After image fetch function is obtained, it's given some wrappers
depending on flags controlling ECR and the presence of a docker config
file:
https://github.com/fluxcd/flux/blob/v1.18.0/cmd/fluxd/main.go#L534

The **image database** is initialised to a no-op implementation:
https://github.com/fluxcd/flux/blob/v1.18.0/cmd/fluxd/main.go#L561

Then (in the lines following) if the **image database** is enabled, a
memcached client is created and the image database constructed around
that; at the same time, the "cache warmer" (**image scanner**
component) is created. This shares the memcached client with the
database. Neither component relies on anything other than flag values,
at this point (the image fetch function is supplied later).

The **automation queue** is comingled with the daemon and sync
component, but essentially it needs:

 - a job queue, which is shared with the API processing (which will
   put jobs on the queue)
 - access to the git repository (actually in the closure of the jobs,
   which are funcs)
 - a goroutine which goes through the jobs and processes them
 - a status cache, which keeps track of the result of jobs, since
   otherwise it mmust be recovered by looking in the git notes (and
   this is expensive).

These are constructed in the lines following
https://github.com/fluxcd/flux/blob/v1.18.0/cmd/fluxd/main.go#L630;
first the git repo, then the job queue. The cache for job results is
constructed inline, in the daemon component:
https://github.com/fluxcd/flux/blob/v1.18.0/cmd/fluxd/main.go#L719.

Some interlinking is done between the **image scanner** component and
other bits of Flux:

 - the **image scanner** is given a function for notifying the
   automation queue about images with new tags (which will typically
   trigger an automation calculation);
 - the **image scanner** is given a channel for telling it to scan a
   specific image repo (which is wired to a webhook receiver);
 - the **image scanner** is started (it runs a loop) and given the
   image fetch function from way above.

#### Dependents and dependencies

 * there's a method for notifying the daemon about new images,
   supplied to the image scanner (as `warmer.Notify`)
 * channel for telling the scanner to look at a specific image repo
   (from webhook), shared with the daemon
 * the sync engine and the automation queue are both driven by a
   single loop, and interact: when a job completes successfully, the
   git repo is refreshed (i.e., does git fetch), which may trigger a
   sync indirectly
   https://github.com/fluxcd/flux/blob/v1.18.0/pkg/daemon/loop.go#L155
   (this is an optimisation of sorts; it may be adequate to leave this
   to webhooks).

#### API surfaces

The HTTP API:

This is actually a conglomerate of automation API, some status
queries, and some syncing queries. Many can be dropped, if
backward-compatiblity can be broken (e.g., leave sync API to the sync
component).

    %%% MORE

Configuration:

The flags and requirements for configuring these components are also a
form of API. There are these bits:

 - the flags;
 - any files that need to be present (mounted from secrets or
   whatever) and their formats;
 - the phase space of how to deploy it (combinations of deployments,
   configmaps and what have you).

    %%% MORE

## Backward compatibility

This will need different configuration, so that is in itself **not**
backward-compatible. It's desirable to keep the distance to the new
configuration small, though, so it makes sense to use the existing
flags such that a mechanical translation is at least in principle
possible.

The aggregate behaviour of this and other components may need to be
backward-compatible, depending on where this lands in the roadmap. For
example, if it's intended that this is usable with an incarnation of
Flux v1, the API served must be backward-compatible.

Otherwise, there is no requirement to be backward-compatible -- this
is a new thing.

## Drawbacks and limitations

As a matter of practicality, the new component will inherit some
drawbacks and limitations inherent in code it reuses from the current
codebase. These may include:

    %%% TODO

## Alternatives

    %%% TODO

**Leave it alone**

**Make the Flux v1 daemon do this (by disabling its syncing?)**

**What else?**

## Unresolved questions

 - is it a requirement that this could be run with a (suitably
   configured) Flux v1 daemon?
 - is there a more ArgoCD-centric design possible (at least to be
   discussed as an alternative)? In particular, what is the
   interaction with custom resources (and can it come later)
