# Get started

> **🛑 Upgrade Advisory**
>
> This documentation is for Flux (v1) which has [reached its end-of-life in November 2022](https://fluxcd.io/blog/2022/10/september-2022-update/#flux-legacy-v1-retirement-plan).
>
> We strongly recommend you familiarise yourself with the newest Flux and [migrate as soon as possible](https://fluxcd.io/flux/migration/).
>
> For documentation regarding the latest Flux, please refer to [this section](https://fluxcd.io/flux/).

All you need is a Kubernetes cluster and a git repo. The git repo
contains [manifests](https://kubernetes.io/docs/concepts/configuration/overview/)
(as YAML files) describing what should run in the cluster. Flux imposes
[some requirements](requirements.md) on these files.

## Installing Flux

Here are the instructions to [install Flux on your own
cluster](tutorials/get-started.md).

If you are using Helm, we have a [separate section about
this](tutorials/get-started-helm.md).
