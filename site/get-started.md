---
title: Installing Weave Flux Manually
menu_order: 10
---

- [Get started with Flux](#get-started-with-flux)
  * [Prerequisites](#prerequisites)
    + [A Note on GKE with RBAC enabled](#a-note-on-gke-with-rbac-enabled)
  * [Set up Flux](#set-up-flux)
  * [Deploying Flux to the cluster](#deploying-flux-to-the-cluster)
  * [Giving write access](#giving-write-access)
  * [Committing a small change](#committing-a-small-change)
  * [Confirm the change landed](#confirm-the-change-landed)
  * [Conclusion](#conclusion)

# Get started with Flux

This short guide shows a self-contained example of Flux and just
takes a couple of minutes to get set up. By the end you will
have Flux running in your cluster and it will be deploying any
code changes for you.

_Note:_ If you would like to install Flux using Helm, refer to the
[Helm section](./helm-get-started.md).

## Prerequisites

You will need to have Kubernetes set up. For a quick local test,
you can use `minikube` or `kubeadm`. Any other Kubernetes setup
will work as well though.

### A Note on GKE with RBAC enabled

> If working on e.g. GKE with RBAC enabled, you will need to add a clusterrolebinding:
>
> ```sh
> kubectl create clusterrolebinding "cluster-admin-$(whoami)" --clusterrole=cluster-admin --user="$(gcloud config get-value core/account)"
> ```
> to avoid an error along the lines of
>
> `Error from server (Forbidden): error when creating "deploy/flux-account.yaml":
> clusterroles.rbac.authorization.k8s.io "flux" is forbidden: attempt to grant
> extra privileges:`

## Set up Flux

Get Flux:

```sh
git clone https://github.com/weaveworks/flux
cd flux
```

Now you can go ahead and edit Flux's deployment manifest. At the very
least you will have to change the `--git-url` parameter to point to
the config repository for the workloads you want Flux to deploy for
you. You are going to need access to this repository.

```sh
$EDITOR deploy/flux-deployment.yaml
```

In our example we are going to use
[flux-get-started](https://github.com/weaveworks/flux-get-started). If you
want to use that too, be sure to create a fork of it on Github and
add the git URL to the config file above.

## Deploying Flux to the cluster

In the next step, deploy Flux to the cluster:

```sh
kubectl apply -f deploy
```

Allow some time for all containers to get up and running. If you're
impatient, run the following command and see the pod creation
process.

```sh
watch kubectl get pods --all-namespaces
```

## Giving write access

At startup Flux generates a SSH key and logs the public key. Find
the SSH public key by installing [fluxctl](./fluxctl.md) and
runnning:

```sh
fluxctl identity
```

In order to sync your cluster state with git you need to copy the
public key and create a deploy key with write access on your GitHub
repository.

Open GitHub, navigate to your fork, go to **Setting > Deploy keys**,
click on **Add deploy key**, give it a name, check **Allow write
access**, paste the Flux public key and click **Add key**.

(Or replace `YOURUSER` with your Github ID in this url:
`https://github.com/YOURUSER/flux-get-started/settings/keys/new` and
paste the key there.)

## Committing a small change

In this example we are using a simple example of a webservice and
change its configuration to use a different message. The easiest
way is to edit your fork of `flux-get-started` and change the `PODINFO_UI_COLOR` env var to `blue`.

Replace `YOURUSER` in
`https://github.com/YOURUSER/flux-get-started/blob/master/workloads/podinfo-dep.yaml`
with your Github ID), open the URL in your browser, edit the file,
change the env var value and commit the file.

You can check out the Flux logs with:

```sh
kubectl -n default logs deployment/flux -f
```

The default sync frequency is 5 minutes. This can be tweaked easily.
By observing the logs you can see when the change landed in in the
cluster.

## Confirm the change landed

To access our webservice and check out its welcome message, simply
run:

```sh
kubectl -n demo port-forward deployment/podinfo 9898:9898 &
```

Open your browser and navigate to `http://localhost:9898`.

## Conclusion

As you can see, the actual steps to set up Flux, get our app
deployed, give Flux access to it and see modifications land are
very straight-forward and are a quite natural work-flow.

As a next step, you might want to dive deeper into [how to
control Flux](./fluxctl.md), check out [more sophisticated
setups](./standalone-setup.md) or go through our hands-on
tutorial about driving Flux, e.g. [automations, annotations and locks](annotations-tutorial.md).