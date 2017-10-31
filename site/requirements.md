---
title: Requirements and limitations
menu_order: 60
---

Flux has some requirements of the files it finds in your git repo.

 * Flux can only deal with one such repo at a time. This limitation is
   technical and may go away.

 * Flux only deals with YAML files at present.

 * A controller resource (e.g., Deployment, DaemonSet, CronJob, or
   StatefulSet) you want to automate must be in its own file. This is
   again a technical limitation, that comes from how the file update
   code works, and may at some point be improved upon.

 * All Kubernetes resource manifests should explicitly specify the
   namespace in which you want them to run. Otherwise, the
   conventional default (`"default"`) will be assumed.

It is _not_ a requirement that the files are arranged in any
particular way into directories. Flux will look in subdirectories for
YAML files recursively, but does not infer any meaning from the
directory structure.
