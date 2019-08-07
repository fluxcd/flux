# Frequently asked questions

### I'm using SSL between Helm and Tiller. How can I configure Flux to use the certificate?

When installing Flux, you can supply the CA and client-side certificate using the `helmOperator.tls` options,
more details [here](https://github.com/fluxcd/flux/blob/master/chart/flux/README.md#installing-weave-flux-helm-operator-and-helm-with-tls-enabled).

### I've deleted a HelmRelease file from Git. Why is the Helm release still running on my cluster?

Flux doesn't delete resources, there is an [issue](https://github.com/fluxcd/flux/issues/738) opened about this topic on GitHub.
In order to delete a Helm release first remove the file from Git and afterwards run:

```yaml
kubectl delete helmrelease/my-release
```

The Helm operator will receive the delete event and will purge the Helm release.

### I've manually deleted a Helm release. Why is Flux not able to restore it?

If you delete a Helm release with `helm delete my-release`, the release name can't be reused.
You need to use the `helm delete --purge` option only then Flux will be able reinstall a release.

### I have a dedicated Kubernetes cluster per environment and I want to use the same Git repo for all. How can I do that?

*Option 1*
For each cluster create a directory in your config repo.
When installing Flux Helm chart set the Git path using `--set git.path=k8s/cluster-name`
and set a unique label for each cluster `--set git.label=cluster-name`.

You can have one or more shared dirs between clusters. Assuming your shared dir is located
at `k8s/common` set the Git path as `--set git.path="k8s/common\,k8s/cluster-name"`.

*Option 2*
For each cluster create a Git branch in your config repo.
When installing Flux Helm chart set the Git branch using `--set git.branch=cluster-name`
and set a unique label for each cluster `--set git.label=cluster-name`.

### Are there prerelease builds I can run?

There are builds from CI for each merge to master branch. See
[fluxcd/helm-operator-prerelease](https://hub.docker.com/r/fluxcd/helm-operator-prerelease/tags).
