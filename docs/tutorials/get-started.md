# Get started with Flux

This short guide shows a self-contained example of Flux and just
takes a couple of minutes to get set up. By the end you will
have Flux running in your cluster and it will be deploying any
code changes for you.

> **Note:** If you would like to install Flux using Helm, refer to the
> [Helm section](get-started-helm.md).

## Prerequisites

You will need to have Kubernetes set up. For a quick local test,
you can use `minikube` or `kubeadm`. Any other Kubernetes setup
will work as well though.

### A note on GKE with RBAC enabled

If working on e.g. GKE with RBAC enabled, you will need to add a clusterrolebinding:

```sh
kubectl create clusterrolebinding "cluster-admin-$(whoami)" --clusterrole=cluster-admin --user="$(gcloud config get-value core/account)"
```

to avoid an error along the lines of:

```sh
Error from server (Forbidden): error when creating "deploy/flux-account.yaml":
clusterroles.rbac.authorization.k8s.io "flux" is forbidden: attempt to grant
extra privileges:
```

## Set up Flux

Get Flux:

```sh
git clone https://github.com/fluxcd/flux
cd flux
```

Now you can go ahead and edit Flux's deployment manifest. At the very
least you will have to change the `--git-url` parameter to point to
the config repository for the workloads you want Flux to deploy for
you. You are going to need access to this repository.

```sh
$EDITOR deploy/flux-deployment.yaml
```

In our example we are going to use [`fluxcd/flux-get-started`](https://github.com/fluxcd/flux-get-started).
If you want to use that too, be sure to create a fork of it on GitHub
and add the git URL to the config file above. After that, set the
`--git-path` flag to `--git-path=namespaces,workloads`, this is meant
to exclude Helm manifests. Again, if you want to get started with Helm,
please refer to the ["Get started with Flux using Helm"](get-started-helm.md)
tutorial.

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
the SSH public key by installing [fluxctl](../references/fluxctl.md) and
running:

```sh
fluxctl identity
```

In order to sync your cluster state with git you need to copy the
public key and create a deploy key with write access on your GitHub
repository.

Open GitHub, navigate to your fork, go to **Setting > Deploy keys**,
click on **Add deploy key**, give it a `Title`, check **Allow write
access**, paste the Flux public key and click **Add key**. See the
[GitHub docs](https://developer.github.com/v3/guides/managing-deploy-keys/#deploy-keys)
for more info on how to manage deploy keys.

(Or replace `YOURUSER` with your GitHub ID in this url:
`https://github.com/YOURUSER/flux-get-started/settings/keys/new` and
paste the key there.)

> **Note:** the SSH key must be configured to have R/W access to the
> repository. More specifically, the SSH key must be able to create
> and update tags. E.g. in Gitlab, that means it requires `Maintainer`
> permissions. The `Developer` permission can create tags, but not
> update them.

## Committing a small change

In this example we are using a simple example of a webservice and
change its configuration to use a different message. The easiest
way is to edit your fork of `flux-get-started` and change the `PODINFO_UI_COLOR` env var to `blue`.

Replace `YOURUSER` in
`https://github.com/YOURUSER/flux-get-started/blob/master/workloads/podinfo-dep.yaml`
with your GitHub ID), open the URL in your browser, edit the file,
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
curl localhost:9898
```

Notice the updated `color` value in the JSON reply.

## Conclusion

As you can see, the actual steps to set up Flux, get our app
deployed, give Flux access to it and see modifications land are
very straight-forward and are a quite natural work-flow.

As a next step, you might want to dive deeper into [how to
control Flux](../references/fluxctl.md), or go through our
hands-on tutorial about driving Flux, e.g.
[automations, annotations and locks](driving-flux.md).
