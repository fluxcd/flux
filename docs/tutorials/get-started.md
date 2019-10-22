# Get started with Flux

This short guide shows a self-contained example of Flux and just
takes a couple of minutes to get set up. By the end you will
have Flux running in your cluster and it will be deploying any
code changes for you.

> **Note:** If you would like to install Flux using Helm, refer to the
> [Helm section](get-started-helm.md).

## Prerequisites

You will need to have Kubernetes set up. For a quick local test,
you can use `minikube`, `kubeadm` or `kind`. Any other Kubernetes setup
will work as well though.

If working on e.g. GKE with RBAC enabled, you will need to add a `ClusterRoleBinding`:

```sh
kubectl create clusterrolebinding "cluster-admin-$(whoami)" \
--clusterrole=cluster-admin \
--user="$(gcloud config get-value core/account)"
```

## Set up Flux

In our example we are going to use
[flux-get-started](https://github.com/fluxcd/flux-get-started). If you
want to use that too, be sure to create a fork of it on GitHub.

First, please [install `fluxctl`](../references/fluxctl.md).

Create the `flux` namespace:

```sh
kubectl create ns flux
```

Then, install Flux in your cluster (replace `YOURUSER` with your GitHub username):

```sh
export GHUSER="YOURUSER"
fluxctl install \
--git-user=${GHUSER} \
--git-email=${GHUSER}@users.noreply.github.com \
--git-url=git@github.com:${GHUSER}/flux-get-started \
--git-path=namespaces,workloads \
--namespace=flux | kubectl apply -f -
```

`--git-path=namespaces,workloads`, is meant to exclude Helm
manifests. Again, if you want to get started with Helm, please refer to the
[Helm section](get-started-helm.md).

Wait for Flux to start:

```sh
kubectl -n flux rollout status deployment/flux
```

### Using a configuration file

You can also use a configuration file to configure Flux instead of specifying command line arguments:

```sh
echo "git-url: git@github.com:${GHUSER}/flux-get-started" >> flux-config.yaml
fluxctl install --config-file=flux-config.yaml --namespace=flux | kubectl apply -f -
```

The configuration file accepts any of the command-line
arguments. Command-line arguments override configuration file
values. By default, a Kubernetes Secret will be created to store the
Flux configuration. If you prefer to use a ConfigMap to specify the
Flux configuration, use the following:

```sh
fluxctl install --config-file=flux-config.yaml --config-as-configmap=true --namespace=flux | kubectl apply -f -
```

## Giving write access

At startup Flux generates a SSH key and logs the public key. Find
the SSH public key by installing [`fluxctl`](../references/fluxctl.md) and
running:

```sh
fluxctl identity --k8s-fwd-ns flux
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
change its configuration to use a different message.

Replace `YOURUSER` in
`https://github.com/YOURUSER/flux-get-started/blob/master/workloads/podinfo-dep.yaml`
with your GitHub ID), open the URL in your browser, edit the file,
change the `PODINFO_UI_MESSAGE` env var to `Welcome to Flux` and commit the file.

By default, Flux git pull frequency is set to 5 minutes.
You can tell Flux to sync the changes immediately with:

```sh
fluxctl sync --k8s-fwd-ns flux
```

## Confirm the change landed

To access our webservice and check out its welcome message, simply
run:

```sh
kubectl -n demo port-forward deployment/podinfo 9898:9898 &
curl localhost:9898
```

Notice the updated `message` value in the JSON reply.

## Conclusion

As you can see, the actual steps to set up Flux, get our app
deployed, give Flux access to it and see modifications land are
very straight-forward and are a quite natural work-flow.

As a next step, you might want to dive deeper into [how to
control Flux](../references/fluxctl.md), or go through our
hands-on tutorial about driving Flux, e.g.
[automations, annotations and locks](driving-flux.md).
