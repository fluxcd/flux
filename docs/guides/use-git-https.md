# Using Git over HTTPS

Instead of making use of Flux' capabilities to generate an SSH private
key, or [supplying your own](provide-own-ssh-key.md), it is possible to
set environment variables and use these in your `--git-url` argument to
provide your HTTPS basic auth credentials without having to expose them
as a plain value in your workload.

> **Note:** setting an HTTP(S) URL as `--git-url` will disable the
> generation of a private key and prevent the setup of the SSH keyring.

> **Note:** the variables _must be escaped with `$()`_ for Kubernetes
> to pass the values to the Flux container, e.g. `$(GIT_AUTHKEY)`.
> [Read more about this Kubernetes feature](https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-container/#using-environment-variables-inside-of-your-config).

1. create a Kubernetes secret with two environment variables and their
   respective values (replace `<username>` and `<token/password>`):

   ```sh
   kubectl create secret generic flux-git-auth --from-literal=GIT_AUTHUSER=<username> --from-literal=GIT_AUTHKEY=<token/password>
   ```

   this will result in a secret that has the structure:

   ```yaml
   apiVersion: v1
   data:
     GIT_AUTHKEY: <base64 encoded token/password>
     GIT_AUTHUSER: <base64 encoded username>
   kind: Secret
   type: Opaque
   metadata:
     ...
   ```

1. mount the Kubernetes secret as environment variables using `envFrom`
   and use them in your `--git-url` argument:

   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: flux
   ...
   spec:
     containers:
     - name: flux
       envFrom:
       - secretRef:
           name: flux-git-auth
       args:
       - --git-url=https://$(GIT_AUTHUSER):$(GIT_AUTHKEY)@<USER>/flux-get-started.git
       ...
   ```