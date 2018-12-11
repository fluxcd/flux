---
title: Introducing Weave Flux
menu_order: 10
---

Continuous delivery is a term that encapsulates a set of best practices 
that surround building, deploying and monitoring applications. The 
goal is to provide a sustainable model for maintaining and improving 
an application.

The promise of continuous delivery relies upon automation and in recent 
years the automation of building and testing software has become 
commonplace. But it is comparatively difficult to automate the 
deployment and monitoring of an application.
[Weave Cloud](https://cloud.weave.works) fixes this problem.

Weave Flux is a tool that automates the deployment of containers to 
Kubernetes. It fills the automation void that exists between building
and monitoring.

## Automated git->cluster synchronisation

Flux's main feature is the automated synchronisation between a version
control repository and a cluster. If you make any changes to your
repository, those changes are automatically deployed to your cluster.

This is a simple, but dramatic improvement on current state of the art.

- All configuration is stored within version control and is inherently
  up to date. At any point anyone could completely recreate the cluster
  in exactly the same state.
- Changes to the cluster are immediately visible to all interested
  parties.
- During a postmortem, the git log provides the perfect history for an
  audit.
- End to end, code to production pipelines become not only possible, but
  easy.

## Automated deployment of new container images

Another feature is the automated deployment of containers. It will
continuously monitor a range of container registries and deploy new
versions where applicable.

This is really useful for keeping the repository and therefore the
cluster up to date. It allows separate teams to have their own
deployment pipelines then Flux is able to see the new image and update
the cluster accordingly.

This feature can be disabled and images can be locked to a specific
version.

## Integrations with other devops tools

One final high-level feature is that Flux increases visibility of your
application. Clear visibility of the state of a cluster is key for
maintaining operational systems. Developers can be confident in their
changes by observing a predictable series of deployment events.

Flux can send notifications to a service (e.g., [Weave
Cloud](https://cloud.weave.works/)) to provide integrations with Slack
and other such media.

## Next

_Find out more about [Flux's features](/site/how-it-works.md)._

_Get started immediately by [installing Flux](/site/installing.md)._
