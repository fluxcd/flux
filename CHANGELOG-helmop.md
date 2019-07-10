## 0.10.0 (2019-07-10)

This release brings you [opt-in automated rollback support][rollback docs],
new Prometheus metrics, and _experimental_ support of spawning
multiple workers with the `--workers=<num>` flag to speed up the
processing of releases.

This will likely also be the last _minor_ beta release before we
promote the Helm operator to its first GA `1.0.0` release.

> **Notice:** the Helm operator relies on changes in the `HelmRelease`
> [Custom Resource Definition][helm 0.10.0 crd]. Please make sure you patch the
> CRD in your cluster _before_ upgrading the Helm operator.

### Bug fixes

 - Prevent an infinite release loop when multiple `HelmRelease`
   resources with the same release name configuration coexist,
   by looking at the antecedent annotation set on release resources
   and confirming ownership
   [weaveworks/flux#2123][#2123]

### Improvements

 - Opt-in automated rollback support; when enabled, a failed release
   will be rolled back automatically and the operator will not attempt
   a new release until it detects a change in the chart and/or the
   configured values
   [weaveworks/flux#2006][#2006]
 - Increase timeout for annotating resources from a Helm release, to
   cope with large umbrella charts
   [weaveworks/flux#2123][#2123]
 - New Prometheus metrics

   + `release_queue_length_count`
   + `release_duration_seconds{action=['INSTALL','UPGRADE'], dry-run=['true', 'false'], success=['true','false'], namespace, releasename}`
   
   [weaveworks/flux#2191][#2191]
 - Experimental support of spawning multiple queue workers processing
   releases by configuring the `--workers=<num>` flag
   [weaveworks/flux#2194][#2194]

### Maintenance and documentation

 - Publish images to [fluxcd DockerHub][] organization
   [weaveworks/flux#2213][#2213]
 - Document opt-in rollback feature
   [weaveworks/flux#2220][#2220]

### Thanks

Many thanks to @adrian, @2opremio, @semyonslepov, @gtseres, @squaremo, @stefanprodan, @kingdonb, @ncabatoff,
@dholbach, @cristian-radu, @simonmacklin, @hiddeco for contributing to this release.

[#2006]: https://github.com/weaveworks/flux/pull/2006
[#2123]: https://github.com/weaveworks/flux/pull/2123
[#2191]: https://github.com/weaveworks/flux/pull/2191
[#2194]: https://github.com/weaveworks/flux/pull/2194
[#2213]: https://github.com/weaveworks/flux/pull/2213
[#2220]: https://github.com/weaveworks/flux/pull/2220
[helm 0.10.0 crd]: https://github.com/weaveworks/flux/blob/release/helm-0.10.x/deploy-helm/flux-helm-release-crd.yaml
[rollback docs]: https://github.com/weaveworks/flux/blob/release/helm-0.10.x/site/helm-integration.md#rollbacks
[fluxcd DockerHub]: https://hub.docker.com/r/weaveworks/helm-operator/

## 0.9.2 (2019-06-13)

### Bug fixes

 - Ensure releases are enqueued on clone change only
   [weaveworks/flux#2081][#2081]
 - Reorder start of processes on boot and verify informer cache sync
   early, to prevent the operator from hanging on boot
   [weaveworks/flux#2103][#2103]
 - Use openssh-client rather than openssh in container image
   [weaveworks/flux#2142][#2142]

### Improvements

 - Enable pprof to ease profiling
   [weaveworks/flux#2095][#2095]

### Maintenance and documentation

 - Add notes about production setup Tiller
   [weaveworks/flux#2146][#2146]
   
### Thanks

Thanks @2opremio, @willholley ,@runningman84, @stefanprodan, @squaremo,
@rossf7, @hiddeco for contributing.

[#2081]: https://github.com/weaveworks/flux/pull/2081
[#2095]: https://github.com/weaveworks/flux/pull/2095
[#2103]: https://github.com/weaveworks/flux/pull/2103
[#2142]: https://github.com/weaveworks/flux/pull/2142
[#2146]: https://github.com/weaveworks/flux/pull/2146

## 0.9.1 (2019-05-09)

### Bug fixes

 - During the lookup of `HelmRelease`s for a mirror, ensure the
   resource has a git chart source before comparing the mirror name
   [weaveworks/flux#2027][#2027]

### Thanks

Thanks to @puzza007, @squaremo, @2opremio, @stefanprodan, @hiddeco
for reporting the issue, patching and reviewing it.

[#2027]: https://github.com/weaveworks/flux/pull/2027

## 0.9.0 (2019-05-08)

### Bug fixes

 - Make sure client-go logs to stderr
   [weaveworks/flux#1945][#1945]
 - Prevent garbage collected `HelmRelease`s from getting upgraded
   [weaveworks/flux#1906][#1906]

### Improvements

 - Enqueue release update on git chart source changes and improve
   mirror change calculations
   [weaveworks/flux#1906][#1906], [weaveworks/flux#2005][#2005]
 - The operator now checks if the `HelmRelease` spec has changed after
   it performed a dry-run, this prevents scenarios where it could
   enroll an older revision of a `HelmRelease` while a newer version
   was already known
   [weaveworks/flux#1906][#1906]
 - Stop logging broadcasted Kubernetes events
   [weaveworks/flux#1906][#1906]
 - Log and return early if release is not upgradable
   [weaveworks/flux#2008][#2008]

### Maintenance and documentation

 - Update client-go to `v1.11`
   [weaveworks/flux#1929][#1929]
 - Move images to DockerHub and have a separate pre-releases image repo
   [weaveworks/flux#1949][#1949], [weaveworks/flux#1956][#1956]
 - Support `arm` and `arm64` builds
   [weaveworks/flux#1950][#1950]
 - Retry keyscan when building images, to mitigate for occasional
   timeouts
   [weaveworks/flux#1971][#1971]

### Thanks

Thanks @brezerk, @jpds, @stefanprodan, @2opremio, @hiddeco, @squaremo,
@dholbach, @bboreham, @bricef and @stevenpall for their contributions
to this release, and anyone who I have missed during this manual
labour.

[#1906]: https://github.com/weaveworks/flux/pull/1906
[#1929]: https://github.com/weaveworks/flux/pull/1929
[#1945]: https://github.com/weaveworks/flux/pull/1945
[#1949]: https://github.com/weaveworks/flux/pull/1949
[#1950]: https://github.com/weaveworks/flux/pull/1950
[#1956]: https://github.com/weaveworks/flux/pull/1956
[#1971]: https://github.com/weaveworks/flux/pull/1971
[#2005]: https://github.com/weaveworks/flux/pull/2005
[#2008]: https://github.com/weaveworks/flux/pull/2008

## 0.8.0 (2019-04-11)

This release bumps the Helm API package and binary to `v2.13.0`;
although we have tested and found it to be backwards compatible, we
recommend running Tiller `>=2.13.0` from now on.

### Improvements

 - Detect changes made to git chart source in `HelmRelease`
   [weaveworks/flux#1865][#1865]
 - Cleanup git chart source clone on `HelmRelease` removal
   [weaveworks/flux#1865][#1865]
 - Add `chartFileRef` option to `valuesFrom` to support using a
   non-default values yamel from a git-sourced Helm chart
   [weaveworks#1909][#1909]
 - Reimplement `--git-poll-interval` to control polling interval of
   git mirrors for chart sources
   [weaveworks/flux#1910][#1910]

### Maintenance and documentation

 - Bump Helm API package and binary to `v2.13.0`
   [weaveworks/flux#1828][#1828]
 - Verify scanned keys in same build step as scan
   [weaveworks/flux#1908][#1908]
 - Use Helm operator image from build in e2e tests
   [weaveworks/flux#1910][#1910]

### Thanks

Thanks to @hpurmann, @2opremio, @arturo-c, @squaremo, @stefanprodan,
@hiddeco, and others for their contributions to this release, feedback,
and bringing us one step closer to a GA-release.

[#1828]: https://github.com/weaveworks/flux/pull/1828
[#1865]: https://github.com/weaveworks/flux/pull/1865
[#1908]: https://github.com/weaveworks/flux/pull/1908
[#1909]: https://github.com/weaveworks/flux/pull/1909
[#1910]: https://github.com/weaveworks/flux/pull/1910

## 0.7.1 (2019-03-27)

### Bug fixes

 - Prevent panic on `.spec.values` in `HelmRelease` due to merge
   attempt on uninitialized value
   [weaveworks/flux#1867](https://github.com/weaveworks/flux/pull/1867)

## 0.7.0 (2019-03-25)

### Bug fixes

 - Run signal listener in a goroutine instead of deferring
   [weaveworks/flux#1680](https://github.com/weaveworks/flux/pull/1680)
 - Make chart operations insensitive to (missing) slashes in Helm
   repository URLs
   [weaveworks/flux#1735](https://github.com/weaveworks/flux/pull/1735)
 - Annotating resources outside of the `HelmRelease` namespace
   [weaveworks/flux#1757](https://github.com/weaveworks/flux/pull/1757)

### Improvements

 - The `HelmRelease` CRD now supports a `skipDepUpdate` to instruct the
   operator to not update dependencies for charts from a git source
   [weaveworks/flux#1712](https://github.com/weaveworks/flux/pull/1712)
   [weaveworks/flux#1823](https://github.com/weaveworks/flux/pull/1823)
 - Azure DevOps Git host support
   [weaveworks/flux#1729](https://github.com/weaveworks/flux/pull/1729)
 - The UID of the `HelmRelease` is now used as dry run release name
   [weaveworks/flux#1745](https://github.com/weaveworks/flux/pull/1745)
 - Removed deprecated `--git-poll-interval` flag
   [weaveworks/flux#1757](https://github.com/weaveworks/flux/pull/1757)
 - Sync hook to instruct the operator to refresh Git mirrors
   [weaveworks/flux#1776](https://github.com/weaveworks/flux/pull/1776)
 - Docker image is now based on Alpine `3.9`
   [weaveworks/flux#1801](https://github.com/weaveworks/flux/pull/1801)
 - `.spec.values` in the `HelmRelease` CRD is no longer mandatory
   [weaveworks/flux#1824](https://github.com/weaveworks/flux/pull/1824)
 - With `valuesFrom` it is now possible to load values from secrets,
   config maps and URLs
   [weaveworks/flux#1836](https://github.com/weaveworks/flux/pull/1836)

### Thanks

Thanks to @captncraig, @2opremio, @squaremo, @hiddeco, @endrec, @ahmadiq,
@nmaupu, @samisq, @yinzara, @stefanprodan, and @sarath-p for their
contributions.

## 0.6.0 (2019-02-07)

### Improvements

 - Add option to limit the Helm operator to a single namespace
   [weaveworks/flux#1664](https://github.com/weaveworks/flux/pull/1664)

### Thanks

Without the contributions of @brandon-bethke-neudesic, @errordeveloper,
@ncabatoff, @stefanprodan, @squaremo, and feedback of our
[#flux](https://slack.weave.works/) inhabitants this release would not
have been possible -- thanks to all of you!

## 0.5.3 (2019-01-14)

### Improvements

  - `HelmRelease` now has a `resetValues` field which when set to `true`
    resets the values to the ones built into the chart
    [weaveworks/flux#1628](https://github.com/weaveworks/flux/pull/1628)
  - The operator now exposes a HTTP webserver (by default on port
    `:3030`) with Prometheus metrics on `/metrics` and a health check
    endpoint on `/healthz`
    [weaveworks/flux#1653](https://github.com/weaveworks/flux/pull/1653)

### Thanks

A thousand thanks to @davidkarlsen, @hiddeco, @ncabatoff, @stefanprodan,
@squaremo and others for their contributions leading to this release.

## 0.5.2 (2018-12-20)

### Bug fixes

  - Respect proxy env entries for git operations
    [weaveworks/flux#1556](https://github.com/weaveworks/flux/pull/1556)
  - Reimplement git timeout after accidentally removing it in `0.5.0`
    [weaveworks/flux#1565](https://github.com/weaveworks/flux/pull/1565)
  - Mark `--git-poll-interval` flag as deprecated
    [weaveworks/flux#1565](https://github.com/weaveworks/flux/pull/1565)
  - Only update chart dependencies if a `requirements.yaml` exists
    weaveworks/flux{[#1561](https://github.com/weaveworks/flux/pull/1561), [#1606](https://github.com/weaveworks/flux/pull/1606)}
    
### Improvements

  - `HelmRelease` now has a `timeout` field (defaults to `300s`),
    giving you control over the amount of time it may take for Helm to
    install or upgrade your chart
    [weaveworks/flux#1566](https://github.com/weaveworks/flux/pull/1566)
  - The Helm operator [flag docs](./site/helm-operator.md#setup-and-configuration)
    have been updated
    [weaveworks/flux#1594](https://github.com/weaveworks/flux/pull/1594)
  - Added tests to ensure Helm dependencies update behaviour is always as
    expected
    [weaveworks/flux#1562](https://github.com/weaveworks/flux/pull/1562)

### Thanks

Thanks to @stephenmoloney, @sfrique, @mgazza, @stefanprodan, @squaremo,
@rade and @hiddeco for their contributions.

## 0.5.1 (2018-11-21)

### Bug fixes

  - Helm releases will now stay put when an upgrade fails or the
    Kubernetes API connectivity is flaky, instead of getting purged
    [weaveworks/flux#1530](https://github.com/weaveworks/flux/pull/1530)

### Thanks

Thanks to @sfrique, @brantb and @squaremo for helping document the
issues leading to this bug fix, @stefanprodan for actually squashing
the bug and all others that may have gone unnoticed while writing this
release note.

## 0.5.0 (2018-11-14)

WARNING: this release of the Helm operator is not backward-compatible:

 - It uses a new custom resource `HelmRelease`, and will ignore
   `FluxHelmRelease` resources
 - Some command-line arguments have changed, so the [deployment
   manifests](./deploy-helm/) must also be updated

To use it, you will need to migrate custom resources to the new format
supported by this version. See the [upgrade
guide](./site/helm-upgrading-to-beta.md).

This version of the Helm operator supports HelmRelease custom
resources, which each specify a chart and values to use in a Helm
release, as in previous versions. The main improvement is that you are
now able to specify charts from Helm repos, as well as from git repo,
per resource (rather than a single git repo, which is supplied to the
operator).

### Improvements

All of these were added in
[weaveworks/flux#1382](https://github.com/weaveworks/flux/pull/1382).

See the [Helm operator guide](./site/helm-integration.md) for details.

 - You can now release charts from arbitrary Helm repos
 - You can now release charts from arbitrary git repos

### Thanks

Thanks to @demikl, @dholbach, @hiddeco, @mellana1, @squaremo,
@stefanprodan, @stephenmoloney, @whereismyjetpack and others who made
suggestions, logged problems, and tried out nightly builds.

## 0.4.0 (2018-11-01)

This release improves support for TLS connections to Tiller; in
particular it makes it much easier to get server certificate
verification (`--tiller-tls-verify`) to work.

It also adds the ability to supply additional values to
`FluxHelmRelease` resources by attaching Kubernetes secrets. This
helps with a few use cases:

 - supplying the same default values to several releases
 - providing secrets (e.g., a password) to a chart that expects them as values
 - using values files without inlining them into FluxHelmReleases

**NB** It is advised that you deploy the operator alongside Tiller
v2.10 or more recent. To properly support TLS, the operator now
includes code from Helm v2.10, and this may have difficulty connecting
to older versions of Tiller.

### Bug fixes

 - Make `--tiller-tls-verify` work as intended, by giving better
   instructions, and adding the argument `--tiller-tls-hostname` which
   lets you specify the hostname that TLS should expect in the
   certificate
   [weaveworks/flux#1484](https://github.com/weaveworks/flux/pull/1484)

### Improvements

 - You can now create secrets containing a `values.yaml` file, and
   attach them to a `FluxHelmRelease` as additional values to use
   [weaveworks/flux#1468](https://github.com/weaveworks/flux/pull/1468)

### Thanks

Thanks to @hiddeco, @Smirl, @stefanprodan, @arthurk, @the-fine,
@wstrange, @sfitts, @squaremo, @mpareja, @stephenmoloney,
@justinbarrick, @pcfens for contributions to the PRs and issues
leading to this release, as well as the inhabitants of
[#flux](https://slack.weave.works/) for high-quality, helpful
discussion.

## 0.3.0 (2018-10-24)

This release adds dependency handling to the Helm operator.

**NB** The helm operator will now update dependencies for charts _by
default_, which means you no longer need to vendor them. You can
switch this behaviour off with the flag `--update-chart-deps=false`.

### Bug fixes

 - Improve chance of graceful shutdown
   [weaveworks/flux#1439](https://github.com/weaveworks/flux/pull/1439)
   and
   [weaveworks/flux#1438](https://github.com/weaveworks/flux/pull/1438)
 
### Improvements

 - The operator now runs `helm dep build` for charts before installing
   or upgrading releases. This will use a lockfile if present, and
   update the dependencies according to `requirements.yaml` otherwise
   [weaveworks/flux#1450](https://github.com/weaveworks/flux/pull/1450)
 - A new flag `--git-timeout` controls how long the Helm operator will
   allow for git operations
   [weaveworks/flux#1416](https://github.com/weaveworks/flux/pull/1416)
 - The Helm operator image now includes the Helm command-line client,
   which makes it easier to troubleshoot problems using `kubectl exec`
   (as part of
   [weaveworks/flux#1450](https://github.com/weaveworks/flux/pull/1450))

## 0.2.1 (2018-09-17)

This is a patch release that allows helm-op to recover from a failed release install.
If a chart is broken, Tiller will reserve the name and mark the release as failed. 
If at a later time the chart is fixed, helm-op can't install it anymore because the release name is in use. 
Purging the release after each failed install allows helm-op to keep retrying the install.

- Purge release if install fails
  [weaveworks/flux#1344](https://github.com/weaveworks/flux/pull/1344)

## 0.2.0 (2018-08-23)

In large part this release simplifies and improves the Helm operator
machinery, without changing its effect.

This release drops the `-alpha` suffix, but remains <1.0 and should
(still) be considered unready for production use.

- Use the same git implementation as fluxd, fixing a number of
  problems with SSH known_hosts and git URLs and so on
  [weaveworks/flux#1240](https://github.com/weaveworks/flux/pull/1240)
- Always check that a chart release will be a change, before releasing
  [weaveworks/flux#1254](https://github.com/weaveworks/flux/pull/1254)
- Add validation to the FluxHelmRelease custom resource definition,
  giving the kind the short name `fhr`
  [weaveworks/flux#1253](https://github.com/weaveworks/flux/pull/1253)
- Detect chart release differences more reliably
  [weaveworks/flux#1272](https://github.com/weaveworks/flux/pull/1272)
- Check for more recent versions and report in logs when out of date
  [weaveworks/flux#1276](https://github.com/weaveworks/flux/pull/1276)

See [getting started with
Helm](https://github.com/weaveworks/flux/blob/master/site/helm/get-started.md)
and the [Helm chart
instructions](https://github.com/weaveworks/flux/blob/master/chart/flux/README.md)
for information on installing the Flux with the Helm operator.

## 0.1.1-alpha (2018-07-16)

- Support using TLS connections to Tiller
  [weaveworks/flux#1200](https://github.com/weaveworks/flux/pull/1200)
- Avoid continual, spurious installs in newer Kubernetes
  [weaveworks/flux#1193](https://github.com/weaveworks/flux/pull/1193)
- Make it easier to override SSH config (and `known_hosts`)
  [weaveworks/flux#1188](https://github.com/weaveworks/flux/pull/1188)
- Annotate resources created by a Helm release with the name of the
  FluxHelmRelease custom resource, so they can be linked
  [weaveworks/flux#1134](https://github.com/weaveworks/flux/pull/1134)
- Purge release when FluxHelmRelease is deleted, so restoring the
  resource can succeed
  [weaveworks/flux#1106](https://github.com/weaveworks/flux/pull/1106)
- Correct permissions on baked-in SSH config
  [weaveworks/flux#1098](https://github.com/weaveworks/flux/pull/1098)
- Test coverage for releasesync package
  [weaveworks/flux#1089](https://github.com/weaveworks/flux/pull/1089)).

It is now possible to install Flux and the Helm operator using the
[helm chart in this
repository](https://github.com/weaveworks/flux/tree/master/chart/flux).

## 0.1.0-alpha (2018-05-01)

First versioned release of the Flux Helm operator. The target features are:

- release Helm charts as specified in FluxHelmRelease resources
  - these refer to charts in a single git repo, readable by the operator
  - update releases when either the FluxHelmRelease resource or the
    chart (in git) changes

See
https://github.com/weaveworks/flux/blob/helm-0.1.0-alpha/site/helm/
for more detailed explanations.
