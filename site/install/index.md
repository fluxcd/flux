# Installing Flux

All you need is a Kubernetes cluster and a git repo. The git repo
contains [manifests][k8s-manifests] (as YAML files) describing what
should run in the cluster. Flux imposes
[some requirements](../requirements.md) on these files.

Here are the instructions to [install Flux on your own
cluster](./get-started.md).

If you are using Helm, we have a [separate section about
this](./helm-get-started.md).

You can also configure a more advanced, [standalone
setup](./standalone-setup.md).

[k8s-manifests]: https://kubernetes.io/docs/concepts/configuration/overview/
