---
title: Installing Weave Flux Manually
menu_order: 10
---

Flux comes in several parts:

-   the command-line client **fluxctl**
-   the daemon, **fluxd** which maintains the state of the cluster

# Quick Install

Install Flux and its dependencies on a kubernetes cluster with:

```
kubectl create -f ./deploy
```

This will create all the resources defined in the
[deploy directory](../deploy/).

Next, download the latest version of the
[fluxctl client from github](https://github.com/weaveworks/flux/releases/latest).

Continue to [setup flux](./setup.md)

---

# Detailed Description

The deployment installs Flux and its dependencies. First, change to the [deploy directory](../deploy/).

```
cd deploy
```

## Memcache

Flux uses memcache to cache docker registry requests.

```
kubectl create -f memcache-dep.yaml memcache-svc.yaml
```

## Flux deployment

The Kubernetes deployment configuration file
[flux-deployment.yaml](../../deploy/flux-deployment.yaml) runs the Flux
service and the Flux daemon in a single pod.

```
kubectl create -f flux-deployment.yaml
```

### Note for Kubernetes >=1.6 with role-based access control (RBAC)

You will need to provide fluxd with a service account which can access
the namespaces you want to use Flux with. To do this, consult the
example service account given in
[flux-account.yaml](../../deploy/flux-account.yaml) (which
puts essentially no constraints on the account) and the
[RBAC documentation](https://kubernetes.io/docs/admin/authorization/rbac/),
and create a service account in whichever namespace you put fluxd
in. You may need to alter the `namespace: default` lines, if you adapt
the example.

You will need to explicitly tell fluxd to use that service account by
uncommenting and possible adapting the line `# serviceAccountName:
flux` in the file `fluxd-deployment.yaml` before applying it.

## Flux Service

To make the pod accessible to the command-line client `fluxctl`, you
can create a service for Flux. The example in `flux-service.yaml`
exposes the service as a
[`NodePort`](http://kubernetes.io/docs/user-guide/services/#type-nodeport).

```
kubectl create -f flux-service.yaml
```

## Fluxctl

This allows you to control Flux from the command line and in standalone
mode is the only way of working with Flux.

Download the latest version of the
[fluxctl client from github](https://github.com/weaveworks/flux/releases/latest).

# Next

Continue to [setup flux](./setup.md)