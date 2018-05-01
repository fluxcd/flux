---
title: Requirements for Helm Integration with Flux (alpha stage)
menu_order: 20
---

# Helm

 - tiller should be running in the cluster, though helm-operator will wait until it can find one.

# Git repo

 - One repo containing both desired release state information and Charts themselves
 - Release state information in the form of Custom Resources manifests is located under a particular path ("releaseconfig" by default; can be overriden)
 - Charts are colocated under another path ("charts" by default; can be overriden). Charts are subdirectories under the charts path.
 - Custom Resource namespace reflects where the release should be done. Both the Helm application and its corresponding Custom Resource will live in this namespace.
 - example of a test repo: https://github.com/tamarakaufler/flux-helm-test

# Custom Resource manifest content
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
