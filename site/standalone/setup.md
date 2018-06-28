---
title: Customising the deployment
menu_order: 20
---

# Customising the deployment

The deployment installs Flux and its dependencies. First, change to
the directory with the examples configuration.

## Memcache

Flux uses memcache to cache docker registry requests.

```sh
kubectl create -f memcache-dep.yaml -f memcache-svc.yaml
```

## Flux deployment

You will need to create a secret in which Flux will store its SSH
key. The daemon won't start without this present.

The `flux` logs should show that it has now connected to the
repository and synchronised the cluster.

When using Kubernetes, this key is stored as a Kubernetes secret. You
can restart `flux` and it will continue to use the same key.

## Add an SSH deploy key to the repository

Flux connects to the repository using an SSH key.

***The SSH key must be configured to have R/W access to the repository***.
 
You have two options:

### 1. Allow flux to generate a key for you.

If you don't specify a key to use, Flux will create one for you. Obtain
the public key through fluxctl:

### 2. Specify a key to use

Create a Kubernetes Secret from a private key:

```sh
kubectl create secret generic flux-git-deploy --from-file=identity=/path/to/private_key
```

The Kubernetes deployment configuration file
[flux-deployment.yaml](../../deploy/flux-deployment.yaml) runs the
Flux daemon, but you'll need to edit it first, at least to supply your
own configuration repo (the `--git-repo` argument).

```sh
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

Using an SSH key allows you to maintain control of the repository. You
can revoke permission for `flux` to access the repository at any time
by removing the deploy key.

## Using a private git host

If you're using your own git host -- e.g., your own installation of
gitlab, or bitbucket server -- you will need to add its host key to
`~/.ssh/known_hosts` in the flux daemon container.

First, run a check that you can clone the repo. The following assumes
that your git server's hostname (e.g., `githost`) is in `$GITHOST` and
the URL you'll use to access the repository (e.g.,
`user@githost:path/to/repo`) is in `$GITREPO`.

```sh
$ # Find the fluxd daemon pod:
$ kubectl get pods --all-namespaces -l name=flux
NAMESPACE   NAME                    READY     STATUS    RESTARTS   AGE
weave       flux-85cdc6cdfc-n2tgf   1/1       Running   0          1h

$ kubectl exec -n weave flux-85cdc6cdfc-n2tgf -ti -- \
    env GITHOST="$GITHOST" GITREPO="$GITREPO" PS1="container$ " /bin/sh

container$ git clone $GITREPO
fatal: Could not read from remote repository

container$ # ^ that was expected. Now we'll try with a modified known_hosts
container$ ssh-keyscan $GITHOST >> ~/.ssh/known_hosts
container$ git clone $GITREPO
Cloning into '...'
...
```

If `git clone` doesn't succeed, you'll need to check that the SSH key
has been installed properly first, then come back. `ssh -vv $GITHOST`
from within the container may help debug it.

If it _did_ work, you will need to make it a more permanent
arrangement. Back in that shell:

```sh
container$ kubectl create configmap flux-ssh-config --from-file=$HOME/.ssh/known_hosts
configmap "flux-ssh-config" created
```

It will be created in the same namespace as the flux daemon, since
you're creating it from within the flux daemon pod.

To use the ConfigMap every time the Flux daemon restarts, you'll need
to mount it into the container. The example deployment manifest
includes an example of doing this, commented out. Uncomment that (it
assumes you used the name above for the ConfigMap) and reapply the
manifest.
You will need to explicitly tell fluxd to use that service account by
uncommenting and possible adapting the line `# serviceAccountName:
flux` in the file `fluxd-deployment.yaml` before applying it.
