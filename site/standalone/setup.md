---
title: Setup Weave Flux Manually
menu_order: 20
---

# Connecting fluxctl to the daemon

You need to tell `fluxctl` where to find the Flux API. If you're using
minikube, say, you can get the IP address of the host, and the port,
with

```
$ flux_host=$(minikube ip)
$ flux_port=$(kubectl get svc flux --template '{{ index .spec.ports 0 "nodePort" }}')
$ export FLUX_URL=http://$flux_host:$flux_port/api/flux
```

Exporting `FLUX_URL` is enough for `fluxctl` to know how to contact
the daemon. You could alternatively supply the `--url` argument each
time.

# Customising the daemon configuration

## Connect flux to a repository

First, you need to connect flux to the repository with Kubernetes
manifests. This is achieved by setting the `--git-url` and
`--git-branch` arguments in the
[`flux-deployment.yaml`](../../deploy/flux-deployment.yaml) manifest.

### Helm users

You need to connect the helm-operator to the same repository, pointing
the helm-operator to the git path containing Charts. This is achieved by
setting the `--git-url` and `--git-branch` arguments to the same values
as for flux and setting the `--git-charts-path` argument in the
[`helm-operator-deployment.yaml`](../../deploy-helm/helm-operator-deployment.yaml)
manifest.

## Add an SSH deploy key to the repository

Flux connects to the repository using an SSH key. You have two
options:

### 1. Allow flux to generate a key for you.

If you don't specify a key to use, Flux will create one for you. Obtain
the public key through fluxctl:

```sh
$ fluxctl identity
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDCN2ECqUFMR413CURbLBcG41fLY75SfVZCd3LCsJBClVlEcMk4lwXxA3X4jowpv2v4Jw2qqiWKJepBf2UweBLmbWYicHc6yboj5o297//+ov0qGt/uRuexMN7WUx6c93VFGV7Pjd60Yilb6GSF8B39iEVq7GQUC1OZRgQnKZWLSQ==
0c:de:7d:47:52:cf:87:61:52:db:e3:b8:d8:1a:b5:ac
+---[RSA 1024]----+
|            ..=  |
|             + B |
|      .     . +.=|
|     . + .   oo o|
|      . S . .o.. |
|           .=.o  |
|           o =   |
|            +    |
|           E     |
+------[MD5]------+
```

Alternatively, you can see the public key in the `flux` log.

The public key will need to be given to the service hosting the Git
repository. For example, in GitHub you would create an SSH deploy key
in the repository, supplying that public key.

The `flux` logs should show that it has now connected to the
repository and synchronised the cluster.

When using Kubernetes, this key is stored as a Kubernetes secret. You
can restart `flux` and it will continue to use the same key.

### 2. Specify a key to use

Create a Kubernetes Secret from a private key:

```
kubectl create secret generic flux-git-deploy --from-file=identity=/path/to/private_key
```

Now add the secret to the `flux-deployment.yaml` manifest:

```
    ...
    spec:
      volumes:
      - name: git-key
        secret:
          secretName: flux-git-deploy
```

And add a volume mount for the container:

```
    ...
    spec:
      containers:
      - name: fluxd
        volumeMounts:
        - name: git-key
          mountPath: /etc/fluxd/ssh
```

You can customise the paths and names of the chosen key with the
arguments (examples with defaults): `--k8s-secret-name=flux-git-deploy`,
`--k8s-secret-volume-mount-path=/etc/fluxd/ssh` and
`--k8s-secret-data-key=identity`

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
