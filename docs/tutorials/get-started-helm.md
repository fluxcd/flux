# Get started with Flux using Helm

If you are using Helm already, this guide is for you. By the end
you will have Helm installing Flux in the cluster and deploying
any code changes for you.

If you are looking for more generic notes for how to install Flux
using Helm, we collected them [in the chart's
README](https://github.com/fluxcd/flux/blob/master/chart/flux/README.md#).

## Prerequisites

You will need to have Kubernetes set up. To get up and running fast,
you might want to use `minikube` or `kubeadm`. Any other Kubernetes
setup will work as well though.

Download Helm:

- On MacOS:

  ```sh
  brew install kubernetes-helm
  ```

- On Linux:
  - Download the [latest release](https://github.com/kubernetes/helm/releases/latest),
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
helm init --skip-refresh --upgrade --service-account tiller --history-max 10
```

> **Note:** This is a quick guide and by no means a production ready
> Tiller setup, please look into ['Securing your Helm installation'](https://helm.sh/docs/using_helm/#securing-your-helm-installation)
> and be aware of the `--history-max` flag before promoting to
> production.

## Install Flux

Add the Flux repository:

```sh
helm repo add fluxcd https://charts.fluxcd.io
```

Apply the Helm Release CRD:

```sh
kubectl apply -f https://raw.githubusercontent.com/fluxcd/helm-operator/master/deploy/flux-helm-release-crd.yaml
```

In this next step you install Flux using `helm`. Simply

 1. Fork [`fluxcd/flux-get-started`](https://github.com/fluxcd/flux-get-started)
    on GitHub and replace the `fluxcd` with your GitHub username in
    [here](https://github.com/fluxcd/flux-get-started/blob/master/releases/ghost.yaml#L13)
 1. Install Flux and the Helm operator by specifying your fork URL:

      *Just make sure you replace `YOURUSER` with your GitHub username
      in the command below:*

    - Using a public git server from `bitbucket.com`, `github.com`, `gitlab.com`, `dev.azure.com`, or `vs-ssh.visualstudio.com`:

      ```sh
      helm upgrade -i flux \
      --set git.url=git@github.com:YOURUSER/flux-get-started \
      --namespace flux \
      fluxcd/flux
      
      helm upgrade -i helm-operator \
      --set git.ssh.secretName=flux-git-deploy \
      --namespace flux \
      fluxcd/helm-operator
      ```

    - Using a private git server:

      When deploying from a private repo, the known_hosts of the git server needs
      to be configured into a kubernetes configmap so that `StrictHostKeyChecking` is respected.
      See the [README of the chart](https://github.com/fluxcd/flux/blob/master/chart/flux/README.md#to-install-flux-with-the-helm-operator-and-a-private-git-repository)
      for further installation instructions in this case.

Allow some time for all containers to get up and running. If you're
impatient, run the following command and see the pod creation
process.

```sh
watch kubectl -n flux get pods
```

You will notice that `flux` and `flux-helm-operator` will start
turning up in the `flux` namespace.

## Giving write access

For the real benefits of GitOps, Flux will need access to your
git repository to update configuration if necessary. To facilitate
that you will need to add a deploy key to your fork of the
repository.

This is pretty straight-forward as Flux generates a SSH key and
logs the public key at startup. Find the SSH public key by
installing [`fluxctl`](../references/fluxctl.md) and running:

```sh
fluxctl identity --k8s-fwd-ns flux
```

In order to sync your cluster state with git you need to copy the
public key and create a deploy key with write access on your GitHub
repository.

Open GitHub, navigate to your fork, go to **Setting > Deploy keys**,
click on **Add deploy key**, give it a `Title`, check **Allow write
access**, paste the Flux public key and click **Add key**.

(Or replace `YOURUSER` with your GitHub ID in this url:
`https://github.com/YOURUSER/flux-get-started/settings/keys/new` and
paste the key there.)

Once Flux has confirmed access to the repository, it will start
deploying the workloads of `flux-get-started`. After a while you
will be able to see the Helm releases deployed by Flux (which are
deployed into the `demo` namespace) listed like so:

```sh
helm list --namespace demo
```

## Committing a small change

`flux-get-started` is a simple example in which three services
(mongodb, redis and ghost) are deployed. Here we will simply update the
version of mongodb to a newer version to see if Flux will pick this
up and update our cluster.

The easiest way is to update your fork of `flux-get-started` and
change the `image` argument.

Replace `YOURUSER` in `https://github.com/YOURUSER/flux-get-started/edit/master/releases/mongodb.yaml`
with your GitHub ID, open the URL in your browser, edit the file,
change the `tag:` line to the following:

```yaml
  values:
    image:
      repository: bitnami/mongodb
      tag: 4.0.6
```

Commit the change to your `master` branch. It will now get
automatically deployed to your cluster.

You can check out the Flux logs with:

```sh
kubectl -n flux logs deployment/flux -f
```

The default sync frequency for Flux using the Helm chart is
five minutes. This can be tweaked easily. By observing the logs
you can see when the change landed in the cluster.

Confirm the change landed by checking the `demo` namespace that
Flux is deploying to:

```sh
kubectl describe -n demo deployment/mongodb | grep Image
```

## Conclusion

As you can see, the actual steps to set up Flux, get our app
deployed, give Flux access to it and see modifications land are
very straight-forward and are a quite natural workflow.

## A more advanced setup

For a more advanced Helm setup, take a look at the
[`fluxcd/helm-operator-get-started` repository](https://github.com/fluxcd/helm-operator-get-started).
