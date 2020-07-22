# Using Git over HTTPS

Instead of making use of Flux' capabilities to generate an SSH private
key, or [supplying your own](provide-own-ssh-key.md), it is possible to
set environment variables and use these in your `--git-url` argument to
provide your HTTPS basic auth credentials without having to expose them
as a plain value in your workload.

!!!note
    Setting an HTTP(S) URL as `--git-url` will disable the
    generation of a private key and prevent the setup of the SSH keyring.

!!!note
    The variables _must be escaped with `$()`_ for Kubernetes
    to pass the values to the Flux container, e.g. `$(GIT_AUTHKEY)`.
    [Read more about this Kubernetes feature](https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-container/#using-environment-variables-inside-of-your-config).

!!!note
    Each of the username and password must be percent-encoded, otherwise
    the git URL may end up being invalid once they have been interpolated
    in. You can encode a string with Perl (assuming your token is in the
    environment variable `TOKEN`):
    
        echo "$TOKEN" | perl -MURI::Escape -ne 'chomp;print uri_escape($_),"\n"'

1. Create a personal access token to be used as the `GIT_AUTHKEY`:

    - [GitHub](https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line)
    - [GitLab](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html#creating-a-personal-access-token)
    - [BitBucket](https://confluence.atlassian.com/bitbucketserver/personal-access-tokens-939515499.html)

1. Create a Kubernetes secret with two environment variables and their
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

1. Mount the Kubernetes secret as environment variables using `envFrom`
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
        # Replace `github.com/...` with your git repository 
        - --git-url=https://$(GIT_AUTHUSER):$(GIT_AUTHKEY)@github.com/fluxcd/flux-get-started.git
        ...
    ```
