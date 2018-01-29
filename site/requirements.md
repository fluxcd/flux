---
title: Requirements and limitations
menu_order: 60
---

Flux has some requirements of the files it finds in your git repo.

 * Flux can only deal with one such repo at a time. This limitation is
   technical and may go away.

 * Flux only deals with YAML files at present. It preserves comments
   and whitespace in YAMLs when updating them.

 * A controller resource (e.g., Deployment, DaemonSet, CronJob, or
   StatefulSet) you want to automate must be in its own file. This is
   again a technical limitation, that comes from how the file update
   code works, and may at some point be improved upon.

 * Flux doesn't understand Kubernetes List resources. This is yet
   again a technical limitation and should get fixed at some point. In
   the meantime, you can just put the resources in the list into their
   own documents (or files, if you want them automated, as above).

 * All Kubernetes resource manifests should explicitly specify the
   namespace in which you want them to run. Otherwise, the
   conventional default (`"default"`) will be assumed.

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
