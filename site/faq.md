---
title: Weave Flux FAQ
menu_order: 60
---

## General questions

Also see [the introduction](/site/introduction.md).

### What does Flux do?

Flux automates the process of deploying containers to Kubernetes.

### How does it automate deployment?

It continuously monitors for new container images, deploys them to 
Kubernetes and saves the resultant configuration in Git.

### How is that different from a bash script?

The amount of functionality contained within Flux warrants a dedicated
application/service. An equivalent script would be too large to
maintain.

### Why should I automate deployment?

Automation is a principle of lean development. It reduces waste, to 
provide efficiency gains. It empowers employees by removing dull
tasks. It mitigates against failure by avoiding silly mistakes.

## Technical questions

### Why does Flux need a deploy key?

Flux needs a deploy key to be allowed to push to the version control
system in order to store the current cluster configuration.

### How do I give Flux access to a private registry?

Provide Flux with the registry credentials. See 
[an example here](/site/using.md).

### How often does Flux check for new images?

Flux polls the registry every 60 s.

