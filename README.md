# Flux v1

This repository contains the source code of Flux Legacy (v1).

Flux v1 has reached **end of life** and has been replaced by [fluxcd/flux2](https://github.com/fluxcd/flux2)
and its controllers entirely.

If you consider using Flux, please take a look at <https://fluxcd.io/flux/get-started>
to get started with v2.

If you are on Flux Legacy, please follow the [migration guide](https://fluxcd.io/flux/migration).
If you need hands-on help migrating, you can contact one of the companies
[listed here](https://fluxcd.io/support/#my-employer-needs-additional-help).

## History

Flux was initially developed by [Weaveworks](https://weave.works) and made open source in 2016.
Over the years the community around Flux & GitOps grew and in 2019 Weaveworks decided to donate
the project to [CNCF](https://cncf.io).

After joining CNCF, the Flux project has seen [massive adoption](https://fluxcd.io/adopters/)
by various organisations. With adoption came a wave of feature requests
that required a major overhaul of Flux monolithic code base and security stance.

In April 2020, the Flux team decided to redesign Flux from the ground up using modern
technologies such as Kubernetes controller runtime and Custom Resource Definitions.
The decision was made to break Flux functionally into specialised components and APIs
with a focus on extensibility, observability and security.
These components are now called the [GitOps Toolkit](https://fluxcd.io/flux/components/).

In 2021, the Flux team launched Flux v2 with many long-requested features like
support for multi-tenancy, support for syncing an arbitrary number of Git repositories,
better observability and a solid security stance. The new version made it possible
to extend Flux capabilities beyond its original GitOps design. The community rallied
around the new design, with an overwhelming number of early adopters and
new contributions, Flux v2 gained new features at a very rapid pace.

In 2022, Flux v2 underwent several security audits, and its code base and APIs became stable
and production ready. Having a dedicated UI for Flux was the most requested feature since we
started the project. For v2, Weaveworks launched a free and open source Web UI for Flux called
[Weave GitOps](https://github.com/weaveworks/weave-gitops), which made Flux and the GitOps
practices more accessible.

Today Flux is an established continuous delivery solution for Kubernetes,
[trusted by organisations](https://fluxcd.io/adopters/) around the world.
Various vendors like Amazon AWS, Microsoft Azure, VMware, Weaveworks
and others offer [Flux as-a-service](https://fluxcd.io/ecosystem/) to their users.
The Flux team is very grateful to the community who supported us over the
years and made Flux what it is today.
