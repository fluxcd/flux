# Flux

Flux is a tool that automatically ensures that the state of a cluster matches the config in git.
It uses an operator in the cluster to trigger deployments inside Kubernetes, which means you don't need a separate CD tool.
It monitors all relevant image repositories, detects new images, triggers deployments and updates the desired running
configuration based on that (and a configurable policy).

## Introduction

This chart bootstraps a [Flux](https://github.com/weaveworks/flux) deployment on
a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.9+

## Installation

We put together a simple [Get Started
guide](../../site/helm/get-started.md) which takes about 5-10 minutes to follow.
You will have a fully working Flux installation deploying workloads to your
cluster.

If you are looking for more generic notes about installation options, you can find
them [here](../../site/helm/installation.md).