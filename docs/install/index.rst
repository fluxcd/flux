======================
Prerequisites for Flux
======================

All you need is a Kubernetes cluster and a git repo. The git repo
contains `manifests <https://kubernetes.io/docs/concepts/configuration/overview/>`_
(as YAML files) describing what should run in the cluster. Flux imposes
:doc:`some requirements <../requirements>` on these files.

Installing Weave Flux
=====================

Here are the instructions to :doc:`install Flux on your own
cluster <./get-started>`.

If you are using Helm, we have a :doc:`separate section about
this <./helm-get-started>`.

You can also configure a more advanced, :doc:`standalone
setup <./standalone-setup>`.

.. toctree::
   :maxdepth: 1
   :caption: Contents:

   get-started
   helm-get-started
   standalone-setup
