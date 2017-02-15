---
title: How Weave Flux Works
menu_order: 70
---

This page describes the goals of flux, how it achieves them and 
significant architectural decisions. It is intentionally high level 
to prevent it from being out of date too quickly.

# Goals

The overall goal of Flux is to automate the deployment of services.
A typical use case would be:

1. A developer merges their tested code into a branch which creates an
   artifact as a (Docker) container image
2. Flux detects the presence of a new image and deploys that to an
   orchestrator (e.g. Kubernetes)
3. Flux writes the new cluster configuration to version control
4. Changes are made visible through chat integration and audit logging
 
Hence, the goal is to automate away the need for a developer to 
interact with an orchestrator (which is a common source of accidental
failure) or with the systems that ensure that the orchestrator is in
a working state.

Flux also provides a CLI and a UI (in Weave Cloud) to perform these
operations manually. Flux is flexible enough to fit into any development
process.

# Implementation Overview

The following describes how Flux achieves the goals.

## Monitoring For New Images

Flux reads a list of running containers from Kubernetes. 
For each image, it will querey the container registry to obtain
the most recently released tag.

You can then observe whether containers are running the most recent
version and then update them to a specific or most recent tag.

When services are in an "automated" mode, the service will 
periodically check to see whether there are any new images. If there 
are, they are deployed.

In order to access private registries, credentials may be required.

## Deployment of Images

Flux will only deploy different images. It will not re-deploy images 
with the same tag.
 
Once a list of new images have been established, it will alter the 
configuration of the cluster to deploy the new images.

## Integration with Version Control

Whenever Flux alters the configuration of a cluster it will write the
changes back to version control.

The state that Flux reports is the state of the 
cluster. If you make changes to the version control, Flux will not 
deploy that new configuration.

A version control deploy key is required to write back to the 
repository.

## Visibility

Flux integrates with Slack. A Slack API endpoint is required for 
integration.

Flux also exposes the history of its internal "job/worker" mechanism 
for auditing purposes. 

# Future changes

## Monitoring for New Images

- _Ability to specify which tags to watch_
- _See integration with version control, may use VCS as source of 
state, not Kubernetes_

## Integration with Version Control

- _VCS will become the source of state_
- _Changes in the VCS will be acted upon by Flux_
