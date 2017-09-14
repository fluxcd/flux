---
title: Flux Daemon
menu_order: 90
---

# Summary

Flux daemon (fluxd, aka flux agent) allows automation of application deployments and version control of cluster configuration.
Version controlling of cluster manifests provides reproducibility and a historical trail of events.

## Flux daemon responsibilities

    A) Continuous Deployment
	    1.
        Flux daemon monitors user git repo Kubernetes manifests for changes, which it
        then deploys to the cluster.

	    2.
	    Flux daemon monitors container registry for running container image updates.
        Detection of an image change (running container image tag vs container
        registry image tag) triggers k8s manifest update, which is committed to the
        user git repository, then deployed to the Kubernetes cluster.

    B) Deployment approaches
        1.
        Automate vs Deautomate
            Deployment happens automatically when a new image tag is detected.
            Deautomated deployment will not proceed until manually released (through
            the UI or the CLI tool fluxctl).
            
        2.
        Lock vs Unlock
            Deployment is pinned to a particular image tag. New deployment will not
            proceed upon triggered release.
    
# Flags

fluxd requires setup and offers customization though a multitude of flags.

|flag                    | default                       | purpose |
|------------------------|-------------------------------|---------|
|--listen -l             | `:3030`                         | Listen address where /metrics and API will be served|
|--kubernetes-kubectl    |                               | Optional, explicit path to kubectl tool|
|--version               | false                         | Get version number|
|**Git repo & key etc.** |                              ||
|--git-url               |                               | URL of git repo with Kubernetes manifests; e.g., `git@github.com:weaveworks/flux-example`|
|--git-branch            | `master`                        | branch of git repo to use for Kubernetes manifests|
|--git-path              |                               | path within git repo to locate Kubernetes manifests (relative path)|
|--git-user              | `Weave Flux`                    | username to use as git committer|
|--git-email             | `support@weave.works`           | email to use as git committer|
|--git-set-author        | false                         | if set, the author of git commits will reflect the user who initiated the commit and will differ from the git committer|
|--git-label             |                               | label to keep track of sync progress; overrides both --git-sync-tag and --git-notes-ref|
|--git-sync-tag          | `flux-sync`             | tag to use to mark sync progress for this cluster (old config, still used if --git-label is not supplied)|
|--git-notes-ref         | `flux`            | ref to use for keeping commit annotations in git notes|
|--git-poll-interval     | `5 minutes`                 | period at which to poll git repo for new commits|
|**registry**            |                               | |
|--memcached-hostname    |                               | hostname for memcached service to use when caching chunks; if empty, no memcached will be used|
|--memcached-timeout     | `1 second`                   | maximum time to wait before giving up on memcached requests|
|--memcached-service     | `memcached`                     | SRV service used to discover memcache servers|
|--registry-cache-expiry | `20 minutes`                  | Duration to keep cached registry tag info. Must be < 1 month.|
|--registry-poll-interval| `5 minutes`                   | period at which to poll registry for new images|
|--registry-rps          | 200                           | maximum registry requests per second per host|
|--registry-burst        | `125`      | maximum number of warmer connections to remote and memcache|
|**k8s-secret backed ssh keyring configuration**      |  | |
|--k8s-secret-name       | `flux-git-deploy`               | name of the k8s secret used to store the private SSH key|
|--k8s-secret-volume-mount-path | `/etc/fluxd/ssh`         | mount location of the k8s secret storing the private SSH key|
|--k8s-secret-data-key   | `identity`                      | data key holding the private SSH key within the k8s secret|
|--connect               |                               | connect to an upstream service e.g., Weave Cloud, at this base address|
|--token                 |                               | authentication token for upstream service|
|**SSH key generation**  |                               | |
|--ssh-keygen-bits       |                               | -b argument to ssh-keygen (default unspecified)|
|--ssh-keygen-type       |                               | -t argument to ssh-keygen (default unspecified)|

