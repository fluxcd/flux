# Requirements and limitations

Flux has some requirements of the files it finds in your git repo.

 * Flux can only deal with one such repo at a time. This limitation is
   technical and may go away.

 * Flux only deals with YAML files at present. It tries to preserve
   comments and whitespace in YAMLs when updating them. You may see
   updates with incidental, harmless changes, like reindented blocks.

 * All Kubernetes resource manifests should explicitly specify the
   namespace in which you want them to run. Otherwise, the
   conventional default (`"default"`) will be assumed.

 * Flux will ignore directories that look like Helm charts, to avoid
   applying templated YAML manifests. A directory will be skipped if
   its contents include the files `Chart.yaml` and `values.yaml`, as
   these are the (only) mandatory components of a Helm chart.

It is _not_ a requirement that the files are arranged in any
particular way into directories. Flux will look in subdirectories for
YAML files recursively, but does not infer any meaning from the
directory structure.

Flux uses the Docker Registry API to collect metadata about the images
running in the cluster. This comes with at least one limitation:

 * Since Flux runs in a container in your cluster, it may not be able
   to resolve all hostnames that you or Kubernetes can resolve. In
   particular, it won't be able to get image metadata for images in a
   private image registry that's made available at `localhost`.
