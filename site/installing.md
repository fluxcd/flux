---
title: Installing Weave Flux
menu_order: 30
---

We recommend that you install Flux with Weave Cloud, our hosted service
for accelerating cloud native development. Using Flux in conjunction
with
[Weave Cloud](https://www.weave.works/solution/cloud/) has the following
benefits:

* A comprehensive dashboard, allowing control of Flux without the CLI
  application
* Extra features not available in the open source version
* Tight integration with other Weave Cloud services
  ([Scope](https://www.weave.works/solution/troubleshooting-dashboard/)
  and
  [Cortex](https://www.weave.works/solution/prometheus-monitoring/))
* Fully hosted and managed by experts at Weaveworks
* Simpler install and operation
* Enterprise support

# Installing via Weave Cloud

Sign up with [Weave Cloud](https://cloud.weave.works) and create an
instance to represent your cluster.

If you're already using Scope or Cortex to look at a cluster, you can
choose that instance instead of creating one. But make sure that this
instance is pointing to the same physical cluster, or else Flux and
Cortex will show conflicting information (e.g. the containers running).

Click on the "Deploy" button and follow the instructions to install
flux.

# Standalone

Alternatively, you can [install flux without Weave Cloud on your own
cluster](./standalone/installing.md).

# Next

[Setup fluxctl](./using.md)