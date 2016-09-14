# Deploying Fluxy to Kubernetes

You will need to build or load the weaveworks/fluxy image into the Docker daemon,
 since the deployment does not attempt to pull the image from a registry.
If you're using [minikube](https://github.com/kubernetes/minikube) to try things locally,
 for example, you can do

```
eval $(minikube docker-env)
make clean build
```

which will build the image in minikube's Docker daemon,
 thus making it available to Kubernetes.

## Creating a key for automation

The automation component mutates a Git repository containing your
Kubernetes config, which requires an SSH access key.  That private key
is stored as a Kubernetes secret named `fluxy-repo-key`.

Here is an example of setting this up for the `helloworld` example in
the fluxy repository.

Fork the fluxy repository on github (you may also wish to rename it,
e.g., to `fluxy-testconf`). Now, we're going to add a deploy key so
fluxy can push to that repo. Generate a key in the console:

```
ssh-keygen -t rsa -b 4096 -f id-rsa-fluxy
```

This makes a private key file (`id-rsa-fluxy`) which we'll supply to
Fluxy in a minute, and a public key file (`id-rsa-fluxy.pub`) which
we'll now give to Github.

On the Github page for your forked repo, go to the settings and find
the "Deploy keys" page. Add one, and paste in the contents of the
`id-rsa-fluxy.pub` file -- the public key.

Kubernetes wants the private key in the form of a secret. To create that,

```
kubectl delete secret fluxy-repo-key
kubectl create secret generic fluxy-repo-key --from-file=id-rsa=id-rsa-fluxy
```

## Customising the deployment config

The file `fluxy-deployment.yaml` contains a Kubernetes deployment
configuration that runs the latest image of Fluxy.

You will need to change at least the repository arguments, to use your
Github repo. Adapt these lines:

```
        - --repo-url=git@github.com:squaremo/fluxy-testdata
        - --repo-key=/var/run/secrets/fluxy/key/id-rsa
        - --repo-path=testdata
```

The last, `--repo-path`, refers to the directory _within_ the
repository containing the configuration files (the setting above is
correct if you have forked the fluxy repo as described so far).

You can create the deployment now:

```
kubectl create -f fluxy-deployment.yaml
```

To make the pod accessible to `fluxctl`, you can create a service for Fluxy and use the Kubernetes API proxy to access it:

```
kubectl create -f fluxy-service.yaml
kubectl proxy &
export FLUX_URL=http://localhost:8001/api/v1/proxy/namespaces/default/services/fluxy
```

This will work with the default settings of `fluxctl`,
 and is especially handy with minikube.

To force Kubernetes to run the latest image after a rebuild, kill the pod:

```
kubectl get pods | grep fluxy | awk '{ print $1 }' | xargs kubectl delete pod
```
