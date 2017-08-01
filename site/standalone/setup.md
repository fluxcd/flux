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
manifests. This is achieved by settings the `--git-url` and
`--git-branch` arguments in the
[`flux-deployment.yaml`](../../deploy/flux-deployment.yaml) manifest.

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
kubectl create secret generic flux-git-deploy --from-file /path/to/identity
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
