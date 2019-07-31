# Providing your own SSH key

Flux connects to the repository using an SSH key it retrieves from a
Kubernetes secret, if the configured (`--k8s-secret-name`) secret has
no `identity` key/value pair, it will generate new private key.

With this knowledge, providing your own SSH key is as simple as
creating the configured secret in the expected format.

1. create a Kubernetes secret from your own private key:

   ```sh
   kubectl create secret generic flux-git-deploy --from-file=identity=/full/path/to/private_key
   ```

   this will result in a secret that has the structure:

   ```yaml
   apiVersion: v1
   data:
     identity: <base64 encoded RSA PRIVATE KEY>
   kind: Secret
   type: Opaque
   metadata:
     ...
   ```
   
2. _(optional)_ if you created the secret with a non-default name
   (default: `flux-git-deploy`), set the `--k8s-secret-name` flag to
   the name of your secret (i.e. `--k8s-secret-name=foo`).

> **Note:** the SSH key must be configured to have R/W access to the
> repository. More specifically, the SSH key must be able to create
> and update tags. E.g. in Gitlab, that means it requires `Maintainer`
> permissions. The `Developer` permission can create tags, but not
> update them.
