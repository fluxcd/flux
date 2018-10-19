---
title: Installing Weave Flux
menu_order: 30
---

# Prerequisites for Flux

All you need is a Kubernetes cluster and a git repo. The git repo
contains [manifests][k8s-manifests] (as YAML files) describing what
should run in the cluster. Flux imposes
[some requirements](/site/requirements.md) on these files.

# Installing Weave Flux

Here are the instructions to [install Flux on your own
cluster](./get-started.md).

If you are using Helm, we have a [separate section about
this](./helm-get-started.md).

# Next

[Setup fluxctl and run the daemon](./using.md)

----
[k8s-manifests]: https://kubernetes.io/docs/concepts/configuration/overview/
