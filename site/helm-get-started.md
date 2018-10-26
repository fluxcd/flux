---
title: Installing Weave Flux using Helm
menu_order: 20
---

# Get started with Flux using Helm

If you are using Helm already, this guide is for you. By the end
you will have Helm installing Flux in the cluster and deploying
any code changes for you.

If you are looking for more generic notes for how to install Flux
using Helm, we collected them [in the chart's
README](../chart/flux/README.md).

## Prerequisites

You will need to have Kubernetes set up. To get up and running fast,
you might want to use `minikube` or `kubeadm`. Any other Kubernetes
setup will work as well though.

*Note:* If you are using `minikube`, be sure to start the
cluster with `--bootstrapper=kubeadm` so you are using RBAC.

Download Helm:

- On MacOS:

  ```sh
  brew install kubernetes-helm
  ```

- On Linux:
  - Download the [latest
    release](https://github.com/kubernetes/helm/releases/latest),
    unpack the tarball and put the binary in your `$PATH`.

Now create a service account and a cluster role binding for Tiller:

```sh
kubectl -n kube-system create sa tiller

kubectl create clusterrolebinding tiller-cluster-rule \
    --clusterrole=cluster-admin \
    --serviceaccount=kube-system:tiller
```

Deploy Tiller in `kube-system` namespace:

```sh
helm init --skip-refresh --upgrade --service-account tiller
```

## Install Weave Flux

Add the Flux repository of Weaveworks:

```sh
helm repo add weaveworks https://weaveworks.github.io/flux
```

In this next step you install Weave Flux using `helm`. Simply

 1. Fork [flux-helm-test](https://github.com/weaveworks/flux-helm-test)
    on Github and
 1. Install Weave Flux and its Helm Operator by specifying your fork
    URL:

      *Just make sure you replace `YOURUSER` with your GitHub username
      in the command below:*
      
    - Using a public git server from `bitbucket.com`, `github.com` or `gitlab.com`:
    
      ```sh
      helm install --name flux \
      --set helmOperator.create=true \
      --set git.url=ssh://git@github.com/YOURUSER/flux-helm-test \
      --set helmOperator.git.chartsPath=charts \
      --namespace flux \
      weaveworks/flux
      ```
      
    - Using a private git server:
       
      When deploying from a private repo, the known_hosts of the git server needs 
      to be configured into a kubernetes configmap so that `StrictHostKeyChecking` is respected.
      See [chart/flux/README.md](https://github.com/weaveworks/flux/blob/master/chart/flux/README.md#to-install-flux-with-the-helm-operator-and-a-private-git-repository)
      for further installation instructions in this case.

Allow some time for all containers to get up and running. If you're
impatient, run the following command and see the pod creation
process.

```sh
watch kubectl get pods --all-namespaces
```

You will notice that `flux` and `flux-helm-operator` will start
turning up in the `flux` namespace.

## Giving write access

For the real benefits of GitOps, Flux will need acccess to your
git repository to update configuration if necessary. To facilitate
that you will need to add a deploy key to your fork of the
repository.

This is pretty straight-forward as Flux generates a SSH key and
logs the public key at startup. Find the SSH public key with:

```sh
FLUX_POD=$(kubectl get pods --namespace flux -l "app=flux,release=flux" -o jsonpath="{.items[0].metadata.name}")
kubectl -n flux logs $FLUX_POD | grep identity.pub | cut -d '"' -f2
```

In order to sync your cluster state with git you need to copy the
public key and create a deploy key with write access on your GitHub
repository.

Open GitHub, navigate to your fork, go to **Setting > Deploy keys**,
click on **Add deploy key**, give it a name, check **Allow write
access**, paste the Flux public key and click **Add key**.

(Or replace `YOURUSER` with your Github ID in this url:
`https://github.com/YOURUSER/flux-helm-test/settings/keys/new` and
paste the key there.)

Once Flux has confirmed access to the repository, it will start
deploying the workloads of `flux-helm-test`. After a while you
will be able to see the Helm releases listed like so:

```sh
helm list --namespace test
```

## Committing a small change

`flux-helm-test` is a very simple example in which two services
(mongodb and mariadb) are deployed. Here we will simply update the
version of mongodb to a newer version to see if Flux will pick this
up and update our cluster.

The easiest way is to update your fork of `flux-helm-test` and
change the `image` argument.

Replace `YOURUSER` in `https://github.com/YOURUSER/flux-helm-test/edit/master/releases/mongodb_release.yaml`
with your Github ID, open the URL in your browser, edit the file,
change the `image:` line to the following:

```yaml
 image: bitnami/mongodb:3.7.9-r13
```

Commit the change to your `master` branch. It will now get
automatically deployed to your cluster.

You can check out the Flux logs with:

```sh
kubectl -n flux logs deployment/flux -f
```

The default sync frequency for Flux using the Helm chart is
30 seconds. This can be tweaked easily. By observing the logs
you can see when the change landed in in the cluster.

## Confirm the change landed

To access our webservice and check out its welcome message, simply
run:

```sh
kubectl describe -n test deployment.apps/mongodb-database-mongodb | grep Image
```

## Conclusion

As you can see, the actual steps to set up Flux, get our app
deployed, give Flux access to it and see modifications land are
very straight-forward and are a quite natural work-flow.

# Next

As a next step, you might want to dive deeper into [how to control
Flux](using.md).

For a more advanced Helm setup, take a look at the [gitops-helm
repository](https://github.com/stefanprodan/gitops-helm).
