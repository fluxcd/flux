---
title: Installing Weave Flux Manually
menu_order: 10
---

Flux comes in several parts:

-   the command-line client **fluxctl**
-   the daemon, **flux** which maintains the state of the cluster
-   a service, which runs in Weave Cloud

But you don't need the last to use Flux; you can just run the daemon
and connect to it with the command line client. This page describes
how.

# Quick Install

Clone the Flux repo and edit the example configuration in
[deploy/flux-deployment.yaml](../../deploy/flux-deployment.yaml). Then
create all the resources defined in the
[deploy directory](../../deploy/).

```
$EDITOR ./deploy/flux-deployment.yaml
kubectl apply -f ./deploy
```

Next, download the latest version of the fluxctl client [from github](https://github.com/weaveworks/flux/releases).

Continue to [setup flux](./setup.md)

---

# Detailed Description

The deployment installs Flux and its dependencies. First, change to
the directory with the examples configuration.

```
cd deploy
```

## Memcache

Flux uses memcache to cache docker registry requests.

```
kubectl create -f memcache-dep.yaml -f memcache-svc.yaml
```

## Flux deployment

You will need to create a secret in which Flux will store its SSH
key. The daemon won't start without this present.

```
kubectl create -f flux-secret.yaml
```

The Kubernetes deployment configuration file
[flux-deployment.yaml](../../deploy/flux-deployment.yaml) runs the
Flux daemon, but you'll need to edit it first, at least to supply your
own configuration repo (the `--git-repo` argument).

```
$EDITOR flux-deployment.yaml
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

## Flux API service

To make the pod accessible to the command-line client `fluxctl`, you
will need to expose the API outside the cluster.

A simple way to do this is to piggy-back on `kubectl port-forward`,
assuming you can access the Kubernetes API:

```
fluxpod=$(kubectl get pod -l name=flux -o name | awk -F / '{ print $2; }')
kubectl port-forward "$fluxpod" 10080:3030 &
export FLUX_URL="http://localhost:10080/api/flux"
fluxctl list-controllers --all-namespaces
```

### Local endpoint

**Beware**: this exposes the Flux API, unauthenticated, over an
insecure channel. Do not do this _unless_ you are operating Flux
entirely locally; and arguably, only to try it out.

If you are running Flux locally, e.g., in minikube, you can use a
service with a
[`NodePort`](http://kubernetes.io/docs/user-guide/services/#type-nodeport).

An example manifest is given in
[flux-nodeport.yaml](../../deploy/flux-nodeport.yaml).

Then you can access the API on the `NodePort`, by retrieving the port
number (this example assumes you are using minikube):

```
fluxport=$(kubectl get svc flux --template '{{ index .spec.ports 0 "nodePort" }}')
export FLUX_URL="http://$(minikube ip):$fluxport/api/flux"
fluxctl list-controllers --all-namespaces
```

## fluxctl

This allows you to control Flux from the command line, and if you're
not connecting it to Weave Cloud, is the only way of working with
Flux.

Download the latest version of the fluxctl client
[from github](https://github.com/weaveworks/flux/releases).

# Next

Continue to [setup flux](./setup.md)
