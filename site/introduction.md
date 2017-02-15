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

One of Flux's main features is the automated deployment of containers.
It will continuously monitor a range of container registries and 
deploy new versions where applicable. 

Also, the Kubernetes cluster configuration is automatically 
synchronised to a version control system. This is an anti-fragile 
procedure to mitigate against the accidental or catastrophic failure 
of an application. This also improves visibility, is reproducible and 
provides a historical log of events.

One final high level feature is that Flux increases visibility of 
your application. It provides an audit history for
your deployments and Slack integration for "ChatOps" style 
development.

These features, and more, integrate tightly with the rest of [Weave 
Cloud](https://cloud.weave.works).

_Find out more about [Flux's features](/site/how-it-works.md)._

_Get started immediately by [installing Flux](/site/installing.md)._
