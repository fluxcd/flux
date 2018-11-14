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
