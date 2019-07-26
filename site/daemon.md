---
title: Flux Daemon
menu_order: 90
---

# Summary

Flux daemon (fluxd, aka Flux agent) allows automation of application deployments and version control of cluster configuration.
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

# More information

Setting up and configuring fluxd is discussed in our [standalone setup](./standalone-setup.md)
document.

# Flags

fluxd requires setup and offers customization though a multitude of flags.

| Flag                                             | Default                            | Purpose
| ------------------------------------------------ | ---------------------------------- | ---
| --listen -l                                      | `:3030`                            | listen address where /metrics and API will be served
| --listen-metrics                                 |                                    | listen address for /metrics endpoint
| --kubernetes-kubectl                             |                                    | optional, explicit path to kubectl tool
| --version                                        | false                              | output the version number and exit
| **Git repo & key etc.**
| --git-url                                        |                                    | URL of git repo with Kubernetes manifests; e.g., `git@github.com:weaveworks/flux-get-started`
| --git-branch                                     | `master`                           | branch of git repo to use for Kubernetes manifests
| --git-ci-skip                                    | false                              | when set, fluxd will append `\n\n[ci skip]` to its commit messages
| --git-ci-skip-message                            | `""`                               | if provided, fluxd will append this to commit messages (overrides --git-ci-skip`)
| --git-path                                       |                                    | path within git repo to locate Kubernetes manifests (relative path)
| --git-user                                       | `Weave Flux`                       | username to use as git committer
| --git-email                                      | `support@weave.works`              | email to use as git committer
| --git-set-author                                 | false                              | if set, the author of git commits will reflect the user who initiated the commit and will differ from the git committer
| --git-gpg-key-import                             |                                    | if set, fluxd will attempt to import the gpg key(s) found on the given path
| --git-signing-key                                |                                    | if set, commits made by fluxd to the user git repo will be signed with the provided GPG key. See [Git commit signing](git-commit-signing.md) to learn how to use this feature
| --git-label                                      |                                    | label to keep track of sync progress; overrides both --git-sync-tag and --git-notes-ref
| --git-sync-tag                                   | `flux-sync`                        | tag to use to mark sync progress for this cluster (old config, still used if --git-label is not supplied)
| --git-notes-ref                                  | `flux`                             | ref to use for keeping commit annotations in git notes
| --git-poll-interval                              | `5m`                               | period at which to fetch any new commits from the git repo
| --git-timeout                                    | `20s`                              | duration after which git operations time out
| --git-secret                                     | false                              | if set, git-secret will be run on every git checkout. A gpg key must be imported using  --git-gpg-key-import or by mounting a keyring containing it directly
| **syncing:** control over how config is applied to the cluster
| --sync-interval                                  | `5m`                               | apply the git config to the cluster at least this often. New commits may provoke more frequent syncs
| --sync-garbage-collection                        | `false`                            | experimental: when set, fluxd will delete resources that it created, but are no longer present in git (see [garbage collection](./garbagecollection.md))
| **automation (of image updates):**
| --automation-interval                            | `5m`                               | period at which to check for image updates for automated workloads
| **registry cache:** (none of these need overriding, usually)
| --memcached-hostname                             | `memcached`                        | hostname for memcached service to use for caching image metadata
| --memcached-timeout                              | `1s`                               | maximum time to wait before giving up on memcached requests
| --memcached-service                              | `memcached`                        | SRV service used to discover memcache servers
| --registry-cache-expiry                          | `1h`                               | Duration to keep cached registry tag info. Must be < 1 month.
| --registry-rps                                   | `200`                              | maximum registry requests per second per host
| --registry-burst                                 | `125`                              | maximum number of warmer connections to remote and memcache
| --registry-insecure-host                         | []                                 | registry hosts to use HTTP for (instead of HTTPS)
| --registry-exclude-image                         | `["k8s.gcr.io/*"]`                 | do not scan images that match these glob expressions
| --registry-use-labels                            | `["index.docker.io/weaveworks/*", "index.docker.io/fluxcd/*"]` | use the timestamp (RFC3339) from labels for (canonical) image refs that match these glob expressions
| --docker-config                                  | `""`                               | path to a Docker config file with default image registry credentials
| --registry-ecr-region                            | `[]`                               | allow these AWS regions when scanning images from ECR (multiple values allowed); defaults to the detected cluster region
| --registry-ecr-include-id                        | `[]`                               | include these AWS account ID(s) when scanning images in ECR (multiple values allowed); empty means allow all, unless excluded
| --registry-ecr-exclude-id                        | `[<EKS SYSTEM ACCOUNT>]`           | exclude these AWS account ID(s) when scanning ECR (multiple values allowed); defaults to the EKS system account, so system images will not be scanned
| --registry-require                               | `[]`                               | exit with an error if the given services are not available. Useful for escalating misconfiguration or outages that might otherwise go undetected. Presently supported values: {`ecr`} |
| **k8s-secret backed ssh keyring configuration**
| --k8s-secret-name                                | `flux-git-deploy`                  | name of the k8s secret used to store the private SSH key
| --k8s-secret-volume-mount-path                   | `/etc/fluxd/ssh`                   | mount location of the k8s secret storing the private SSH key
| --k8s-secret-data-key                            | `identity`                         | data key holding the private SSH key within the k8s secret
| **k8s configuration**
| --k8s-allow-namespace                            |                                    | experimental: restrict all operations to the provided namespaces
| **upstream service**
| --connect                                        |                                    | connect to an upstream service e.g., Weave Cloud, at this base address
| --token                                          |                                    | authentication token for upstream service
| **SSH key generation**
| --ssh-keygen-bits                                |                                    | -b argument to ssh-keygen (default unspecified)
| --ssh-keygen-type                                |                                    | -t argument to ssh-keygen (default unspecified)
| **manifest generation**
| --manifest-generation                            | false                              | experimental; search for .flux.yaml files to generate manifests
