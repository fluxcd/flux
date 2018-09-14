# Design overview

Chart release information is described through Kubernetes Custom Resource (CR) manifests.

Flux-Helm Integration implementation consists of two parts:

1. *Flux agent* monitors user git repo containing deployment configuration for applications/services, ie Custom Resource manifests. On detecting changes it applies the manifests, resulting in creation or update of Custom Resources.

2. [*Helm operator*](helm-operator.md) deals with Helm Chart releases. The operator watches for changes of Custom Resources of kind FluxHelmRelease. It receives Kubernetes Events and acts accordingly, installing, upgrading or deleting a Chart release.

## More detail

 - Kubernetes Custom Resource (CR) manifests contain all the information needed to do a Chart release. There is 1-2-1 releationship between a Helm Chart and a Custom Resource.

 - Custom resource manifests can be provided several/all in one file, or in individual files.

 - Flux works, at the moment, with one git repo. For Helm integration this repo will initially contain both the desired Chart release information (CR manifests) and Chart directories for each application/service.

 - All Chart release configuration is located under one git path. All Chart directories are located under one git path. The git paths must be subdirectories under the repo root.

 - [Helm operator](helm-operator.md) uses (Kubernetes) shared informer caching and a work queue, that is processed by a configurable number of workers.

# Custom Resource manifest content

- name of the resource must be unique across all namespaces
- namespace is where both the Custom Resource and the Chart, whose deployment state it describes, will live
- labels.chart must be provided. the label contains this Chart's path within the repo (slash replaced with underscore)
- chartgitpath ... this Chart's path within the repo
- releasename is optional. Must be provided if there is already a Chart release in the cluster that Flux should start looking after. Otherwise a new release is created for the application/service when the Custom Resource is created. Can be provided for a brand new release - if it is not, then Flux will create a release names as $namespace-$CR_name
- customizations section contains user customizations overriding the Chart values


## Example of manifest content

```
---
  apiVersion: helm.integrations.flux.weave.works/v1alpha2
  kind: FluxHelmRelease
  metadata:
    name: mongodb
    namespace:  myNamespace
    labels:
      chart: mongodb
  spec:
    chartGitPath: mongodb
    releaseName: mongo-database
    values:
      image: bitnami/mongodb:3.7.1-r1
```

## Required fields

 - name
 - namespace
 - label.chart  ... the same as chartgitpath, with slash replaced with  an underscore
 - chartGitPath ... path (from repo root) to a Chart subdirectory


## Optional fields

 - releaseName:

  - if a release already exists and Flux should start managing it, then releasename must be provided
  - if releasename is not provided, Flux will construct a release name based on the namespace and the Custom Resource name (ie $namespace-$CR_name)

```
  - values:
      foo: value1
      bar:
        baz: value2
      oof:
        - item1
        - item2
```

  a dictionary of key value pairs (which can be nested) for overriding Chart parameters. Examples of parameter names:

  - image
  - resources -> requests -> memory (nested)
