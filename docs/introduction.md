# Introducing Flux

Continuous delivery is a term that encapsulates a set of best practices
that surround building, deploying and monitoring applications. The
goal is to provide a sustainable model for maintaining and improving
an application.

Flux is a tool that automates the deployment of containers to
Kubernetes. It fills the automation void that exists between building
and monitoring.

## Automated git->cluster synchronisation

Flux's main feature is the automated synchronisation between a version
control repository and a cluster. If you make any changes to your
repository, those changes are automatically deployed to your cluster.

This is a simple, but dramatic improvement on current state of the art.

- All configuration is stored within version control and is inherently
  up to date. At any point anyone could completely recreate the cluster
  in exactly the same state of configuration.
- Changes to the cluster are immediately visible to all interested
  parties.
- During a postmortem, the git log provides the perfect history for an
  audit.
- End to end, code to production pipelines become not only possible, but
  easy.

## Automated deployment of new container images

Another feature is the automated deployment of containers. It will
continuously monitor a range of container registries and deploy new
versions where applicable.

This is really useful for keeping the repository and therefore the
cluster up to date. It allows separate teams to have their own
deployment pipelines since Flux is able to see the new image and update
the cluster accordingly.

This feature can be disabled and images can be locked to a specific
version.

## Integrations with other devops tools

For configuration customization across environments and clusters, Flux comes with builtin support 
for [Kustomize](references/fluxyaml-config-files.md) and [Helm](references/helm-operator-integration.md).

For advanced deployment patterns like Canary releases, A/B testing and Blue/Green deployments,
Flux can be used together with [Flagger](https://github.com/weaveworks/flagger).
