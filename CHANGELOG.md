## 1.3.0 (2018-04-26)

### Fixes

- Exclude no-longer relevant changes from auto-releases [weaveworks/flux#1036](https://github.com/weaveworks/flux/pull/1036)
- Make release and auto-release events more accurately record the
  affected resources, by looking at the calculated result [weaveworks/flux#1050](https://github.com/weaveworks/flux/pull/1050)

### Improvements

- Let the flux daemon operate without a git repo, and report cluster resources as read-only when there is no corresponding manifest [weaveworks/flux#962](https://github.com/weaveworks/flux/pull/962)
- Reinstate command-line arg for setting the git polling interval `--git-poll-interval` [weaveworks/flux#1030](https://github.com/weaveworks/flux/pull/1030)
- Add `--git-ci-skip` (and for more fine control, `--git-ci-skip-message`) for customising flux's commit messages such that CI systems ignore the commits [weaveworks/flux#1011](https://github.com/weaveworks/flux/pull/1011)
- Log the daemon version on startup [weaveworks/flux#1017](https://github.com/weaveworks/flux/pull/1017)

## 1.2.5 (2018-03-19)

### Fixes

- Handle single-quoted image values in manifests [weaveworks/flux#1008](https://github.com/weaveworks/flux/pull/1008)

### Improvements

- Use a writable tmpfs volume for generating keys, since Kubernetes >=1.10 and GKE (as of March 13 2018) mount secrets as read-only [weaveworks/flux#1007](https://github.com/weaveworks/flux/pull/1007)

## 1.2.4 (2018-03-14)

### Fixes

- CLI help examples updated with new resource ID format [weaveworks/flux#945](https://github.com/weaveworks/flux/pull/945)
- Fix a panic caused by accessing a `nil` map when logging events [weaveworks/flux#975](https://github.com/weaveworks/flux/pull/975)
- Properly support multi-line lock messages [weaveworks/flux#978](https://github.com/weaveworks/flux/pull/978)
- Ignore Helm charts when looking for Kubernetes manifests [weaveworks/flux#993](https://github.com/weaveworks/flux/pull/993)

### Improvements

- Enable pprof [weaveworks/flux#927](https://github.com/weaveworks/flux/pull/927/files)
- Use a Kubernetes serviceAccount when deploying Flux standalone [weaveworks/flux#972](https://github.com/weaveworks/flux/pull/972)
- Ensure at-least-once delivery of events to Weave Cloud [weaveworks/flux#973](https://github.com/weaveworks/flux/pull/973)
- Include resource sync errors when logging a sync event [weaveworks/flux#970](https://github.com/weaveworks/flux/pull/970)

### Experimental

- Alpha release of [helm-operator](https://github.com/weaveworks/flux/blob/master/site/helm/helm-integration.md)

## 1.2.3 (2018-02-07)

### Fixes

- Fix a spin loop in the registry cache [weaveworks/flux#928](https://github.com/weaveworks/flux/pull/928)

## 1.2.2 (2018-01-31)

### Fixes

- Correctly handle YAML files with no trailing newline
  [weaveworks/flux#916](https://github.com/weaveworks/flux/issues/916)

### Improvements

The following improvements are to help if you are running a private
registry.

- Support image registries using basic authentication (rather than
  token-based authentication)
  [weaveworks/flux#915](https://github.com/weaveworks/flux/issues/915)
- Introduce the daemon argument `--registry-insecure-host` for marking
  a registry as accessible via HTTP (rather than HTTPS)
  [weaveworks/flux#918](https://github.com/weaveworks/flux/pull/918)
- Better logging of registry fetch failures, for troubleshooting
  [weaveworks/flux#898](https://github.com/weaveworks/flux/pull/898)

## 1.2.1 (2018-01-15)

### Fixes

- Fix an issue that prevented fetching tags for private repositories on DockerHub (and self-hosted registries) [weaveworks/flux#897](https://github.com/weaveworks/flux/pull/897)

## 1.2.0 (2018-01-04)

### Improvements

- Releases are more responsive, because dry runs are now done without triggering a sync [weaveworks/flux#862](https://github.com/weaveworks/flux/pull/862)
- Syncs are much faster, because they are now done all-in-one rather than calling kubectl for each resource [weaveworks/flux#872](https://github.com/weaveworks/flux/pull/872)
- Rewrite of the image registry package to solve several problems [weaveworks/flux#851](https://github.com/weaveworks/flux/pull/851)

### Fixes

- Support signed manifests (from GCR in particular) [weaveworks/flux#838](https://github.com/weaveworks/flux/issues/838)
- Support CronJobs from Kubernetes API version `batch/v1beta1`, which are present in Kubernetes 1.7 (while those from `batch/b2alpha1` are not) [weaveworks/flux#868](https://github.com/weaveworks/flux/issues/868)
- Expand the GCR credentials support to `*.gcr.io` [weaveworks/flux#882](https://github.com/weaveworks/flux/pull/882)
- Check that the synced git repo is writable before syncing, which avoids a number of indirect failures [weaveworks/flux#865](https://github.com/weaveworks/flux/pull/865)
- and, [lots of other things](https://github.com/weaveworks/flux/pulls?q=is%3Apr+closed%3A%3E2017-11-01)

## 1.1.0 (2017-11-01)

### Improvements

- Flux can now release updates to DaemonSets, StatefulSets and
  CronJobs in addition to Deployments. Matching Service resources are
  no longer required.

## 1.0.2 (2017-10-18)

### Improvements

- Implemented support for v2 registry manifests.

## 1.0.1 (2017-09-19)

### Improvements

- Flux daemon can be configured to populate the git commit author with
  the name of the requesting user
- When multiple flux daemons share the same configuration repository,
  each fluxd only sends Slack notifications for commits that affect
  its branch/path
- When a resource is locked the invoking user is recorded, along with
  an optional message
- When a new config repo is synced for the first time, don't send
  notifications for the entire commit history

### Fixes

- The `fluxctl identity` command only worked via the Weave Cloud
  service, and not when connecting directly to the daemon

## 1.0.0 (2017-08-22)

This release introduces significant changes to the way flux works:

- The git repository is now the system of record for your cluster
  state. Flux continually works to synchronise your cluster with the
  config repository
- Release, automation and policy actions work by updating the config
  repository

See https://github.com/weaveworks/flux/releases/tag/1.0.0 for full
details.

## 0.3.0 (2017-05-03)

Update to support newer Kubernetes (1.6.1).

### Potentially breaking changes

- Support for Kubernetes' ReplicationControllers is deprecated; please
  update these to Deployments, which do the same job but much better
  (see
  https://kubernetes.io/docs/user-guide/replication-controller/#deployment-recommended)
- The service<->daemon protocol is versioned. The daemon will now
  crash-loop, printing a warning to the log, if it tries to connect to
  the service with a deprecated version of the protocol.

### Improvements

-   Updated the version of `kubectl` bundled in the Flux daemon image,
    to work with newer (>1.5) Kubernetes.
-   Added `fluxctl save` command for bootstrapping a repo from an existing cluster
-   You can now record a message and username with each release, which
    show up in notifications

## 0.2.0 (2017-03-16)

More informative and helpful UI.

### Features

-   Lots more documentation
-   More informative output from `fluxctl release`
-   Added option in `fluxctl set-config` to generate a deploy key

### Improvements

-   Slack notifications are tidier
-   Support for releasing to >1 service at a time
-   Better behaviour when flux deploys itself
-   More help given for commonly encountered errors
-   Filter out Kubernetes add-ons from consideration
-   More consistent Prometheus metric labeling

See also https://github.com/weaveworks/flux/issues?&q=closed%3A"2017-01-27 .. 2017-03-15"

## 0.1.0 (2017-01-27)

Initial semver release.

### Features

-   Validate image release requests.
-   Added version command

### Improvements

-   Added rate limiting to prevent registry 500's
-   Added new release process
-   Refactored registry code and improved coverage

See https://github.com/weaveworks/flux/milestone/7?closed=1 for full details.

