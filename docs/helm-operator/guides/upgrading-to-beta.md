# Upgrading from Helm operator alpha (<=0.4.0) to beta

The Helm operator has undergone changes that necessitate some changes
to custom resources, and the deployment of the operator itself.

The central difference is that the Helm operator now works with
resources of the kind `HelmRelease` in the API version
`flux.weave.works/v1beta1`, which have a different format to the
custom resources used by the old Helm operator (`FluxHelmRelease`).

Here are some things to know:

- The new operator will ignore the old custom resources (and the old
  operator will ignore the new resources).
- Deleting a resource while the corresponding operator is running
  will result in the Helm release also being deleted
- Deleting a `CustomResourceDefinition` will also delete all
  custom resources of that kind.
- If both operators are running and both new and old custom resources
  defining a release, the operators will fight over the release.

The safest way to upgrade is to avoid deletions and fights by stopping
the old operator. Replacing it with the new one (e.g., by changing the
deployment, or re-releasing the Flux chart with the new version) will
have that effect.

Once the old operator is not running, it is safe to deploy the new
operator, and start replacing the old resources with new
resources. You can keep the old resources around during this process,
since the new operator will ignore them.

## Upgrading the operator deployment

### Using the Flux chart

The chart (from v0.5.0, or from this git repo) provides the
correct arguments to the operator; to upgrade, do

```sh
helm repo update

helm upgrade flux --reuse-values \
--set image.tag=1.8.1 \
--set helmOperator.tag=0.5.1 \
--namespace=flux \
fluxcd/flux --version 0.5.1
```

The chart will leave the old custom resource definition and custom
resources in place. You will need to replace the individual resources,
as described below.

### Using manifests

You will need to adapt any existing manifest that you use to run the
Helm operator. The arguments to the operator executable have changed,
since it no longer needs the git repo to be specified (and in some
cases, just to tidy up):

- the new operator does not use the `--git-url`, `--git-charts-path`,
  or `--git-branch` arguments, since the git repo and so on are
  provided in each custom resource.
- the `--queue-worker-count` argument has been removed
- the `--chart-sync-timeout` argument has been removed
- other arguments stay the same

It is entirely valid to run the operator with no arguments, which you
may end up with after removing those mentioned above. It will work
with the secrets mounted as for the old operator, to start off with,
since it expects the SSH key for the git repo to be in the same place.

Once you want to use the new capabilities of the operator -- e.g.,
releasing charts from Helm repos -- you will probably need to adapt
the manifest further. The [Helm operator set-up
guide](../../references/helm-operator-integration.md) and [example
deployment](https://github.com/fluxcd/flux/blob/master/deploy-helm/helm-operator-deployment.yaml)
explain all the details.

## Updating custom resources

The main differences between the old resource format and the new are:

- the API version and kind have changed
- you can now specify a chart to release either as a path in a git
  repo, or a named, versioned chart from a Helm repo

Here is how to change an old resource to a new resource:

- change the `apiVersion` field to `flux.weave.works/v1beta1`
- change the `kind` field to `HelmRelease`
- you can remove the label `chart:` from the labels, if it's still
  there, just to tidy up (it doesn't matter if it's there or not)
- replace the field `chartGitPath`, with the structure:

```yaml
chart:
  git: <URL to git repo>
  ref: <optional branch name>
  path: <path from top directory of git repo to chart directory>
```

- the `values`, `releaseName`, and `valueFileSecrets` can stay as
  they are.

Note that you now give the git repo URL and branch and full path in
each custom resource, rather than supplying arguments to the Helm
operator. (As you've been using the old operator, you'll only have one
git repo for all charts -- but now you can use different repos for
charts!)

As a full example, this is an old resource:

```yaml
---
apiVersion: helm.integrations.flux.weave.works/v1alpha2
kind: FluxHelmRelease
metadata:
  name: foobar
  namespace: foo-ns
spec:
  chartGitPath: foobar
  values:
    image:
      repository: foobar
      tag: v1
```

Say the arguments given to the old Helm operator were

```yaml
args:
  - --git-url=git@example.com:user/repo
  - --git-charts-path=charts
  - --git-branch=master
```

Then the new custom resource would be:

```yaml
---
apiVersion: flux.weave.works/v1beta1 # <- change API version
kind: HelmRelease                    # <- change kind
metadata:
  name: foobar
  namespace: foo-ns
spec:
  chart:
    git: git@example.com:user/repo # <- --git-url from operator args
    path: charts/foobar            # <- join --git-chart-path and chartGitPath
  values:
    image:
      repository: foobar
      tag: v1
```
