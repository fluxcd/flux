---
title: Installing Weave Flux Manually
menu_order: 10
---

# Get started with Flux

This short guide shows a self-contained example of Flux and just
takes a couple of minutes to get set up. By the end you will
have Flux running in your cluster and it will be deploying any
code changes for you.

_Note:_ If you would like to install Flux using Helm, refer to the
[Helm section](../helm/installing.md).

## Prerequisites

You will need to have Kubernetes set up. For a quick local test,
you can use `minikube` or `kubeadm`. Any other Kubernetes setup
will work as well though.

## Set up Flux

Get Flux:

```sh
git clone https://github.com/weaveworks/flux
cd flux
```

Now you can go ahead and edit Flux's deployment manifes. At the very
least you will have to change the `--git-url` parameter to point to
the config repository for the workloads you want Flux to deploy for
you. You are going to need access to this repository.

```sh
$EDITOR deploy/flux-deployment.yaml
```

In our example we are going to use
[flux-example](https://github.com/weaveworks/flux-example). If you
want to use that too, be sure to create a fork of it on Github and
add the git URL to the config file above.

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
the SSH public key with:

```sh
kubectl logs deployment/flux | grep identity.pub | cut -d '"' -f2
```

In order to sync your cluster state with git you need to copy the
public key and create a deploy key with write access on your GitHub
repository.

Open GitHub, navigate to your fork, go to **Setting > Deploy keys**,
click on **Add deploy key**, give it a name, check **Allow write
access**, paste the Flux public key and click **Add key**.

(Or replace `YOURUSER` with your Github ID in this url:
`https://github.com/YOURUSER/flux-example/settings/keys/new` and
paste the key there.)

## Committing a small change

In this example we are using a simple example of a webservice and
change its configuration to use a different message. The easiest
way is to your fork of `flux-example` and change the `msg` argument.

Replace `YOURUSER` in
`https://github.com/YOURUSER/flux-example/blob/master/helloworld-deploy.yaml`
with your Github ID), open the URL in your browser, edit the file,
change the argument value and commit the file.

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
kubectl port-forward deployment/helloworld 8080:80 &
curl localhost:8080
```

<<<<<<< HEAD
### Remote endpoint

```
export POD_NAME=$(kubectl get pods --namespace default -l "name=flux" -o jsonpath="{.items[0].metadata.name}")
kubectl port-forward $POD_NAME 3030:3030
export FLUX_URL="http://127.0.0.1:3030/api/flux"
fluxctl list-controllers --all-namespaces
```

## fluxctl

This allows you to control Flux from the command line, and if you're
not connecting it to Weave Cloud, is the only way of working with
Flux.

Download the latest version of the fluxctl client
[from github](https://github.com/weaveworks/flux/releases).

## Helm operator (Helm users only)

The Kubernetes deployment configuration file
[helm-operator-deployment.yaml](../../deploy-helm/helm-operator-deployment.yaml) runs the
helm operator, but you'll need to edit it first, at least to supply your
own configuration repo (the `--git-repo` as for flux and the `--git-charts-path`
argument).

```
$EDITOR helm-operator-deployment.yaml
kubectl create -f flux-helm-release-crd.yaml -f helm-operator-deployment.yaml
```
=======
## Conclusion
>>>>>>> a0e4ec3... add self-contained get-started standalone guide

As you can see, the actual steps to set up Flux, get our app
deployed, give Flux access to it and see modifications land are
very straight-forward and are a quite natural work-flow.

As a next step, you might want to dive deeper into [how to control
Flux](site/using.md) or check out [more sophisticated
setups](site/setup.md).