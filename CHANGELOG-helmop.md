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
