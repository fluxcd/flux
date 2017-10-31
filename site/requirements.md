---
title: Requirements and limitations
menu_order: 60
---

Flux has some requirements of the files it finds in your git repo.

 * Flux only deals with YAML files at present.

 * A controller resource (e.g., Deployment, DaemonSet, CronJob, or
   StatefulSet) you want to automate must be in its own file. This is
   a technical limitation that comes from how the file update code
   works, and may at some point cease to be a requirement.

 * All Kubernetes resource manifests should explicitly specify the
   namespace in which you want them to run. Otherwise, the
   conventional default (`"default"`) will be assumed.
