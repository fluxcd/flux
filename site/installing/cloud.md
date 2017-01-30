---
title: Installing Weave Flux with Weave Cloud
menu_order: 10
---

In addition to visualising and monitoring your cloud native application,
[Weave Cloud](https://cloud.weave.works) also provides a fully hosted
version of Flux. The use of weave cloud dramatically simplifies the
installation and ongoing operation of Flux.

Using Flux on [Weave Cloud](https://cloud.weave.works) requires the
installation of two components:

-   the command-line client **fluxctl** and
-   the daemon, **fluxd** which carries out tasks on behalf of the
hosted service.

# Installing

Sign up with [Weave Cloud](https://cloud.weave.works) and create an
instance to represent your cluster. 

If you're already using Scope or Cortex to look at a cluster, you can
choose that instance instead of creating one. But make sure that this
instance is pointing to the same physical cluster, or else Flux and
Cortex will show conflicting information (e.g. the containers running).

Click on the "Deploy" button and follow the instructions to install
flux.