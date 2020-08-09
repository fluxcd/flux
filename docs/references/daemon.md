# Daemon (`fluxd`)

## Summary

Flux daemon (`fluxd`, aka Flux agent) allows automation of application deployments and version control of cluster configuration.
Version controlling of cluster manifests provides reproducibility and a historical trail of events.

### Responsibilities

#### Continuous Deployment

1. Flux daemon monitors user git repo Kubernetes manifests for
   changes, which it then deploys to the cluster.

2. Flux daemon monitors container registry for running container image
   updates. Detection of an image change (running container image tag
   vs container registry image tag) triggers k8s manifest update, which
   is committed to the user git repository, then deployed to the
   Kubernetes cluster.

#### Deployment approaches
        
1. Automate vs Deautomate

    Deployment happens automatically when a new image tag is
    detected. Deautomated deployment will not proceed until
    manually released (through the CLI tool `fluxctl`).

2. Lock vs Unlock

    Deployment is pinned to a particular image tag.
    New deployment will not proceed upon triggered release.

## Setup and configuration

`fluxd` requires setup and offers customization though a multitude of flags.

| Flag                                             | Default                            | Purpose
| ------------------------------------------------ | ---------------------------------- | ---
| --listen -l                                      | `:3030`                            | listen address where /metrics and API will be served
| --listen-metrics                                 |                                    | listen address for /metrics endpoint
| --kubernetes-kubectl                             |                                    | optional, explicit path to kubectl tool
| --version                                        | false                              | output the version number and exit
| **Git repo & key etc.**
| --git-url                                        |                          | URL of git repo with Kubernetes manifests; e.g., `git@github.com:fluxcd/flux-get-started`
| --git-branch                                     | `master`                 | branch of git repo to use for Kubernetes manifests
| --git-ci-skip                                    | false                    | when set, fluxd will append `\n\n[ci skip]` to its commit messages
| --git-ci-skip-message                            | `""`                     | if provided, fluxd will append this to commit messages (overrides --git-ci-skip`)
| --git-path                                       |                          | path within git repo to locate Kubernetes manifests (relative path)
| --git-user                                       | `Weave Flux`             | username to use as git committer
| --git-email                                      | `support@weave.works`    | email to use as git committer
| --git-set-author                                 | false                    | if set, the author of git commits will reflect the user who initiated the commit and will differ from the git committer
| --git-gpg-key-import                             |                          | if set, fluxd will attempt to import the gpg key(s) found on the given path
| --git-signing-key                                |                          | if set, commits made by fluxd to the user git repo will be signed with the provided GPG key.
| --git-secret                                     |                          | if set and a `.gitsecret` directory exist in the root of the git repository, Flux will execute a `git secret reveal -f` in the working clone before performing any operations. mutually exclusive with `--git-crypt`
| --git-crypt                                      |                          | if set and a `.git-crypt` directory exist in the root of the git repository, Flux will execute a `git crypt unlock` in the working clone before performing any operations. mutually exclusive with `--git-secret`
| --git-label                                      |                          | label to keep track of sync progress; overrides both --git-sync-tag and --git-notes-ref
| --git-sync-tag                                   | `flux-sync`              | tag to use to mark sync progress for this cluster (old config, still used if --git-label is not supplied)
| --git-notes-ref                                  | `flux`                   | ref to use for keeping commit annotations in git notes
| --git-poll-interval                              | `5m`                     | period at which to fetch any new commits from the git repo
| --git-timeout                                    | `20s`                    | duration after which git operations time out
| --git-readonly                                   | `false`                  | If `true`, the git repo will be considered read-only, and Flux will not attempt to write to it. Implies --sync-state=secret
| **syncing:** control over how config is applied to the cluster
| --sync-interval                                  | `5m`                     | apply the git config to the cluster at least this often. New commits may provoke more frequent syncs
| --sync-timeout                                   | `1m`                     | duration after which sync operations time out
| --sync-garbage-collection                        | `false`                  | when set, fluxd will delete resources that it created, but are no longer present in git
| --sync-garbage-collection-dry                    | `false`                  | only log what would be garbage collected, rather than deleting. Implies --sync-garbage-collection
| --sync-state                                     | `git`                    | Where to keep sync state; either a tag in the upstream repo (`git`), or as an annotation on the SSH secret (`secret`)
| **registry cache:** (none of these need overriding, usually)
| --memcached-hostname                             | `memcached`                        | hostname for memcached service to use for caching image metadata
| --memcached-timeout                              | `1s`                               | maximum time to wait before giving up on memcached requests
| --memcached-service                              | `memcached`                        | SRV service used to discover memcache servers
| --registry-cache-expiry                          | `1h`                               | Duration to keep cached registry tag info. Must be < 1 month.
| --registry-rps                                   | `200`                              | maximum registry requests per second per host
| --registry-burst                                 | `125`                              | maximum number of warmer connections to remote and memcache
| --registry-insecure-host                         | []                                 | registry hosts to use HTTP for (instead of HTTPS)
| --registry-exclude-image                         | `["k8s.gcr.io/*"]`                 | do not scan images that match these glob expressions
| --registry-include-image                         | `nil`                              | scan _only_ images that match these glob expressions (the default, `nil`, means include everything)
| --registry-use-labels                            | `["index.docker.io/weaveworks/*", "index.docker.io/fluxcd/*"]` | use the timestamp (RFC3339) from labels for (canonical) image refs that match these glob expressions
| --docker-config                                  | `""`                               | path to a Docker config file with default image registry credentials
| --registry-ecr-region                            | `[]`                               | allow these AWS regions when scanning images from ECR (multiple values allowed); defaults to the detected cluster region
| --registry-ecr-include-id                        | `[]`                               | include these AWS account ID(s) when scanning images in ECR (multiple values allowed); empty means allow all, unless excluded
| --registry-ecr-exclude-id                        | `[<EKS SYSTEM ACCOUNT>]`           | exclude these AWS account ID(s) when scanning ECR (multiple values allowed); defaults to the EKS system account, so system images will not be scanned
| --registry-require                               | `[]`                               | exit with an error if the given services are not available. Useful for escalating misconfiguration or outages that might otherwise go undetected. Presently supported values: {`ecr`} |
| --registry-disable-scanning                      | `false`                            | do not scan container image registries to fill in the registry cache
| **k8s-secret backed ssh keyring configuration**
| --k8s-secret-name                                | `flux-git-deploy`                  | name of the k8s secret used to store the private SSH key
| --k8s-secret-volume-mount-path                   | `/etc/fluxd/ssh`                   | mount location of the k8s secret storing the private SSH key
| --k8s-secret-data-key                            | `identity`                         | data key holding the private SSH key within the k8s secret
| **k8s configuration**
| --k8s-allow-namespace                            |                                    | restrict all operations to the provided namespaces
| --k8s-default-namespace                          |                                    | the namespace to use for resources where a namespace is not specified
| --k8s-unsafe-exclude-resource                    | `["*metrics.k8s.io/*", "webhook.certmanager.k8s.io/*", "v1/Event"]` | do not attempt to obtain cluster resources whose group/version/kind matches these glob expressions, e.g. `coordination.k8s.io/v1beta1/Lease`, `coordination.k8s.io/*/Lease` or `coordination.k8s.io/*`. Potentially unsafe, please read Flux's troubleshooting section on `--k8s-unsafe-exclude-resource` before using it.
| **upstream service**
| --connect                                        |                                    | connect to an upstream service e.g., Weave Cloud, at this base address
| --token                                          |                                    | authentication token for upstream service
| **SSH key generation**
| --ssh-keygen-bits                                |                                    | -b argument to ssh-keygen (default unspecified)
| --ssh-keygen-type                                |                                    | -t argument to ssh-keygen (default unspecified)
| --ssh-keygen-format                              |                                    | -m argument to ssh-keygen (default RFC4716)
| **manifest generation**
| --manifest-generation                            | false                              | search for .flux.yaml files to generate manifests
| --sops                                           | false                              | decrypt SOPS-encrypted manifest files before applying them to the cluster. Provide decryption keys in the same way as providing them for `sops` the binary, for example with `--git-gpg-key-import`. The full description of how to supply sops with a key can be found in the [SOPS documentation](https://github.com/mozilla/sops#usage). Be aware that manifests generated with `.flux.yaml` files are not decrypted. Instead, make sure to output cleartext manifests by explicitly invoking the `sops` binary.

## More information

Setting up and configuring `fluxd` is discussed in
["Get started with Flux"](../tutorials/get-started.md).

There is also more information on [garbage collection](garbagecollection.md),
[Git commit signing](git-gpg.md), and other elements in [references](../).
