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
