## 1.2.0 (2020-02-06)

### Improvements

 - Updated Flux to `1.18.0`
   [fluxcd/flux#2825](https://github.com/fluxcd/flux/pull/2825)
 - Add registry disable scanning to chart options
   [fluxcd/flux#2828](https://github.com/fluxcd/flux/pull/2828)
 - Add pod labels to chart options
   [fluxcd/flux#2775](https://github.com/fluxcd/flux/pull/2775)
 - Add sops decryption to chart options 
   [fluxcd/flux#2762](https://github.com/fluxcd/flux/pull/2762)

## 1.1.0 (2020-01-14)

### Improvements

 - Updated Flux to `1.17.1`
   [fluxcd/flux#2738](https://github.com/fluxcd/flux/pull/2738)
 - Separate Git Poll Interval from Sync Interval
   [fluxcd/flux#2721](https://github.com/fluxcd/flux/pull/2721)
 - Namespace whitelisting in helm chart without clusterRole
   [fluxcd/flux#2719](https://github.com/fluxcd/flux/pull/2719)
 - Added hostAliases to deployment template
   [fluxcd/flux#2705](https://github.com/fluxcd/flux/pull/2705)

## 1.0.0 (2019-12-16)

**Note** The Helm Operator manifests have been **removed** from this chart.
Please see the [install instruction](https://github.com/fluxcd/helm-operator/tree/master/chart/helm-operator)
for Helm Operator v1.0.0. To keep using the same SSH key as Flux see the docs
[here](https://github.com/fluxcd/helm-operator/tree/master/chart/helm-operator#use-fluxs-git-deploy-key).
The upgrade procedure for `HelmReleases` from `v1beta1` to `v1` can be found
[here](https://docs.fluxcd.io/projects/helm-operator/en/latest/guides/upgrading-to-ga.html).

### Improvements

 - Updated Flux to `1.17.0`
   [fluxcd/flux#2693](https://github.com/fluxcd/flux/pull/2693)
 - Add a ServiceMonitor template
   [fluxcd/flux#2668](https://github.com/fluxcd/flux/pull/2668)
 - Update the automation interval flag in the chart
   [fluxcd/flux#2551](https://github.com/fluxcd/flux/pull/2551)

## 0.16.0 (2019-11-28)

### Improvements

 - Updated Flux to `1.16.0`
   [fluxcd/flux#2639](https://github.com/fluxcd/flux/pull/2639)
 - Allow `git.verifySignature` to be `"false"`
   [fluxcd/flux#2573](https://github.com/fluxcd/flux/pull/2573)
 - Update the automation interval flag in the chart
   [fluxcd/flux#2551](https://github.com/fluxcd/flux/pull/2551)

### Bug fixes

 - Fix memcached PSP
   [fluxcd/flux#2542](https://github.com/fluxcd/flux/pull/2542)

## 0.15.0 (2019-10-07)

**Note** The Helm Operator options will be **removed** from this chart in the next major release.
Please see the [install instruction](https://github.com/fluxcd/helm-operator/tree/master/chart/helm-operator)
for Helm Operator v1.0.0. To keep using the same SSH key as Flux see the docs
[here](https://github.com/fluxcd/helm-operator/tree/master/chart/helm-operator#use-fluxs-git-deploy-key).
The upgrade procedure for `HelmReleases` from `v1beta1` to `v1` can be found
[here](https://docs.fluxcd.io/projects/helm-operator/en/latest/guides/upgrading-to-ga.html).

### Improvements

 - Updated Flux to `1.15.0`
   [fluxcd/flux#2490](https://github.com/fluxcd/flux/pull/2490)
 - Support secure Git over HTTPS using credentials from environment variables
   [fluxcd/flux#2470](https://github.com/fluxcd/flux/pull/2470)
 - Make sync operations timeout configurable with the `sync.timeout` option
   [fluxcd/flux#2481](https://github.com/fluxcd/flux/pull/2481)

### Bug fixes

 - Mount AKS service principal through secret instead of hostPath for ACR support
   [fluxcd/flux#2437](https://github.com/fluxcd/flux/pull/2437)
   [fluxcd/flux#2434](https://github.com/fluxcd/flux/pull/2434)
   
## 0.14.1 (2019-09-04)

### Improvements

 - Updated Flux to `1.14.2`
   [fluxcd/flux#2419](https://github.com/fluxcd/flux/pull/2419)

## 0.14.0 (2019-08-22)

### Improvements

 - Updated Flux to `1.14.1`
   [fluxcd/flux#2401](https://github.com/fluxcd/flux/pull/2401)
 - Add the ability to disable memcached and set an external memcached service
   [fluxcd/flux#2393](https://github.com/fluxcd/flux/pull/2393)

## 0.13.0 (2019-08-21)

### Improvements

**Note** The Flux chart is now hosted at `https://charts.fluxcd.io`

 - Updated Flux to `1.14.0`
   [fluxcd/flux#2380](https://github.com/fluxcd/flux/pull/2380)
 - Add `git.readonly` option to chart
   [fluxcd/flux#1807](https://github.com/fluxcd/flux/pull/1807)
 - Helm chart repository has been changed to `charts.fluxcd.io`
   [fluxcd/flux#2341](https://github.com/fluxcd/flux/pull/2341)

## 0.12.0 (2019-08-08)

### Improvements

 - Updated Flux to `1.13.3` and the Helm operator to `0.10.1`
   [fluxcd/flux#2296](https://github.com/fluxcd/flux/pull/2296)
   [fluxcd/flux#2318](https://github.com/fluxcd/flux/pull/2318)
 - Add manifest generation to helm chart
   [fluxcd/flux#2332](https://github.com/fluxcd/flux/pull/2332)
   [fluxcd/flux#2335](https://github.com/fluxcd/flux/pull/2335)
 - Let a named cluster role be used in chart
   [fluxcd/flux#2266](https://github.com/fluxcd/flux/pull/2266)

## 0.11.0 (2019-07-10)

### Improvements

 - Updated Flux to `1.13.2` and the Helm operator to `0.10.0`
   [fluxcd/flux#2235](https://github.com/fluxcd/flux/pull/2235)
   [fluxcd/flux#2237](https://github.com/fluxcd/flux/pull/2237)
 - Changed from DockerHub organization `weaveworks` -> `fluxcd`
   [fluxcd/flux#2224](https://github.com/fluxcd/flux/pull/2224)
 - Updated `HelmRelease` CRD to support rollbacks
   [fluxcd/flux#2006](https://github.com/fluxcd/flux/pull/2006)
 - Allow namespace scoping for both Flux and the Helm operator
   [fluxcd/flux#2206](https://github.com/fluxcd/flux/pull/2206)
   [fluxcd/flux#2209](https://github.com/fluxcd/flux/pull/2209)
 - Removed long deprecated `FluxHelmRelease` CRD and disabled CRD
   creation as the default to follow our own best practices
   [fluxcd/flux#2190](https://github.com/fluxcd/flux/pull/2190)
 - Enable `PodSecurityPolicy`
   [fluxcd/flux#2223](https://github.com/fluxcd/flux/pull/2223)
   [fluxcd/flux#2225](https://github.com/fluxcd/flux/pull/2225)
 - Support new Flux `--registry-use-labels` flag (`registry.useTimestampLabels`)
   [fluxcd/flux#2176](https://github.com/fluxcd/flux/pull/2176)
 - Support new Helm operator `--workers` flag (`helmOperator.workers`)
   [fluxcd/flux#2236](https://github.com/fluxcd/flux/pull/2236)

## 0.10.2 (2019-06-27)

### Improvements

 - Updated Flux to `1.13.1`
   [weaveworks/flux#2203](https://github.com/weaveworks/flux/pull/2203)

## 0.10.1 (2019-06-16)

### Bug fixes

 - Fix memcached security context
   [weaveworks/flux#2163](https://github.com/weaveworks/flux/pull/2163)

## 0.10.0 (2019-06-14)

### Improvements

 - Updated Flux to `1.13.0` and Helm operator to `0.9.2`
   [weaveworks/flux#2150](https://github.com/weaveworks/flux/pull/2150)
   [weaveworks/flux#2153](https://github.com/weaveworks/flux/pull/2153)
 - Updated memcached to `1.5.15` and configured default security context
   [weaveworks/flux#2107](https://github.com/weaveworks/flux/pull/2107)
 - Toggle garbage collection dry-run
   [weaveworks/flux#2063](https://github.com/weaveworks/flux/pull/2063)
 - Toggle git signature verification
   [weaveworks/flux#2053](https://github.com/weaveworks/flux/pull/2053)
 - Support `dnsPolicy` and `dnsConfig` in Flux daemon deployment
   [weaveworks/flux#2116](https://github.com/weaveworks/flux/pull/2116)
 - Support configurable log format
   [weaveworks/flux#2138](https://github.com/weaveworks/flux/pull/2138)
 - Support additional sidecar containers
   [weaveworks/flux#2130](https://github.com/weaveworks/flux/pull/2130)

### Bug fixes

 - Fix `extraVolumes` indentation
   [weaveworks/flux#2102](https://github.com/weaveworks/flux/pull/2102)

## 0.9.5 (2019-05-22)

 - Updated Flux to `1.12.3`
   [weaveworks/flux#2076](https://github.com/weaveworks/flux/pull/2076)

## 0.9.4 (2019-05-09)

 - Updated Helm operator to `0.9.1`
   [weaveworks/flux#2032](https://github.com/weaveworks/flux/pull/2032)

## 0.9.3 (2019-05-08)

### Improvements

 - Updated Flux to `1.12.2` and Helm operator to `0.9.0`
   [weaveworks/flux#2025](https://github.com/weaveworks/flux/pull/2025)
 - Mount sub path of repositories secret
   [weaveworks/flux#2014](https://github.com/weaveworks/flux/pull/2014)
 - Toggle garbage collection
   [weaveworks/flux#2004](https://github.com/weaveworks/flux/pull/2004)

## 0.9.2 (2019-04-29)

### Improvements

 - Updated Flux to `1.12.1`
   [weaveworks/flux#1993](https://github.com/weaveworks/flux/pull/1993)

## 0.9.1 (2019-04-17)

### Improvements

 - Add the `status` subresource to HelmRelease CRD
   [weaveworks/flux#1906](https://github.com/weaveworks/flux/pull/1906)
 - Switch image registry from Quay to Docker Hub
   [weaveworks/flux#1949](https://github.com/weaveworks/flux/pull/1949)

## 0.9.0 (2019-04-11)

### Improvements

 - Updated Flux to `1.12.0` and Helm operator to `0.8.0`
   [weaveworks/flux#1924](https://github.com/weaveworks/flux/pull/1924)
 - Add ECR require option
   [weaveworks/flux#1863](https://github.com/weaveworks/flux/pull/1863)
 - Support loading values from alternative files in chart 
   [weaveworks/flux#1909](https://github.com/weaveworks/flux/pull/1909)
 - Add Git poll interval option
   [weaveworks/flux#1910](https://github.com/weaveworks/flux/pull/1910)
 - Add init container, extra volumes and volume mounts
   [weaveworks/flux#1918](https://github.com/weaveworks/flux/pull/1918)
 - Add docker config file path option
   [weaveworks/flux#1919](https://github.com/weaveworks/flux/pull/1919)

## 0.8.0 (2019-04-04)

### Improvements

 - Updated Flux to `1.11.1`
   [weaveworks/flux#1892](https://github.com/weaveworks/flux/pull/1892)
 - Define custom Helm repositories in the Helm chart
   [weaveworks/flux#1893](https://github.com/weaveworks/flux/pull/1893)
 - Increase memcached max memory to 512MB
   [weaveworks/flux#1900](https://github.com/weaveworks/flux/pull/1900)

## 0.7.0 (2019-03-27)

### Improvements

 - Updated Flux to `1.11.0` and Helm operator to `0.7.1`
   [weaveworks/flux#1871](https://github.com/weaveworks/flux/pull/1871)
 - Allow mounting of docker credentials file
   [weaveworks/flux#1762](https://github.com/weaveworks/flux/pull/1762)
 - Increase memcached memory defaults
   [weaveworks/flux#1780](https://github.com/weaveworks/flux/pull/1780)
 - GPG Git commit signing
   [weaveworks/flux#1394](https://github.com/weaveworks/flux/pull/1394)

## 0.6.3 (2019-02-14)

### Improvements

 - Updated Flux to `1.10.1`
   [weaveworks/flux#1740](https://github.com/weaveworks/flux/pull/1740)
 - Add option to set pod annotations
   [weaveworks/flux#1737](https://github.com/weaveworks/flux/pull/1737)

## 0.6.2 (2019-02-11)

### Improvements

 - Allow chart images to be pulled from a private container registry
   [weaveworks/flux#1718](https://github.com/weaveworks/flux/pull/1718)

### Bug fixes

 - Fix helm-op allow namespace flag mapping
   [weaveworks/flux#1724](https://github.com/weaveworks/flux/pull/1724)

## 0.6.1 (2019-02-07)

### Improvements

 - Updated Flux to `1.10.0` and Helm operator to `0.6.0`
   [weaveworks/flux#1713](https://github.com/weaveworks/flux/pull/1713)
 - Add option to exclude container images
   [weaveworks/flux#1659](https://github.com/weaveworks/flux/pull/1659)
 - Add option to mount custom `repositories.yaml`
   [weaveworks/flux#1671](https://github.com/weaveworks/flux/pull/1671)
 - Add option to limit the Helm operator to a single namespace
   [weaveworks/flux#1664](https://github.com/weaveworks/flux/pull/1664)

### Bug fixes

 - Fix custom SSH secret mapping
   [weaveworks/flux#1710](https://github.com/weaveworks/flux/pull/1710)

## 0.6.0 (2019-01-14)

**Note** To fix the connectivity problems between Flux and memcached we've changed the
memcached service from headless to ClusterIP. This change will make the Helm upgrade fail
with `ClusterIP field is immutable`.

Before upgrading to 0.6.0 you have to delete the memcached headless service:

```bash
kubectl -n flux delete svc flux-memcached
```

### Improvements

 - Updated Flux to `1.9.0` and Helm operator to `0.5.3`
   [weaveworks/flux#1662](https://github.com/weaveworks/flux/pull/1662)
 - Add resetValues field to HelmRelease CRD
   [weaveworks/flux#1628](https://github.com/weaveworks/flux/pull/1628)
 - Use ClusterIP service name for connecting to memcached
   [weaveworks/flux#1618](https://github.com/weaveworks/flux/pull/1618)
 - Increase comprehensiveness of values table in `chart/flux/README.md`
   [weaveworks/flux#1626](https://github.com/weaveworks/flux/pull/1626)
    - Rectify error where `resources` are not `None` by default in `chart/flux/values.yaml`
    - Add more fields that are actually in `chart/flux/values.yaml`
    - Separate `replicaCount` into a Flux one and `helmOperator.replicaCount` one
  - Only create the `flux-helm-tls-ca-config` file if `.Values.helmOperator.tls.caContent` exists.
    Useful when doing Flux upgrades but do not happen to know or want to specify
    the `caContent` in `values.yaml`. Otherwise, the existing caContent will be overriden with an
    empty value.
    [weaveworks/flux#1649](https://github.com/weaveworks/flux/pull/1649)
  - Add Flux AWS ECR flags
    [weaveworks/flux#1655](https://github.com/weaveworks/flux/pull/1655)


## 0.5.2 (2018-12-20)

### Improvements

 - Updated Flux to `v1.8.2` and Helm operator to `v0.5.2`
   [weaveworks/flux#1612](https://github.com/weaveworks/flux/pull/1612)
 - Parameterized the memcached image repo
   [weaveworks/flux#1592](https://github.com/weaveworks/flux/pull/1592)
 - Allow existing service account to be provided on helm install
   [weaveworks/flux#1589](https://github.com/weaveworks/flux/pull/1589)
 - Make SSH known hosts volume optional
   [weaveworks/flux#1544](https://github.com/weaveworks/flux/pull/1544)

### Thanks

Thanks to @davidkarlsen, @stephenmoloney, @batpok, @squaremo,
@hiddeco and @stefanprodan for their contributions.

## 0.5.1 (2018-11-21)

### Bug fixes

 - Removed CRD hook from chart
   [weaveworks/flux#1536](https://github.com/weaveworks/flux/pull/1536)

### Improvements

 - Updated Helm operator to `v0.5.1`
   [weaveworks/flux#1536](https://github.com/weaveworks/flux/pull/1536)
 - Updated chart README (removed Helm operator Git flags, fixed typos,
   updated example repo and use the same Git URL format everywhere)
   [weaveworks/flux#1527](https://github.com/weaveworks/flux/pull/1527)

## 0.5.0 (2018-11-16)

### Improvements

 - Updated Flux to `v1.8.1` and the Helm operator to `v0.5.0`
   [weaveworks/flux#1522](https://github.com/weaveworks/flux/pull/1522)
 - Adapted chart to new Helm operator CRD and args
   [weaveworks/flux#1382](https://github.com/weaveworks/flux/pull/1382)

## 0.4.1 (2018-11-04)

### Bug fixes

 - Fixed indentation of `.Values.helmOperator.tls.caContent`
   [weaveworks/flux#1484](https://github.com/weaveworks/flux/pull/1484)

### Improvements

 - Updated Helm operator to `v0.4.0`
   [weaveworks/flux#1487](https://github.com/weaveworks/flux/pull/1487)
 - Added `--tiller-tls-hostname` Helm operator config flag to the chart
   [weaveworks/flux#1484](https://github.com/weaveworks/flux/pull/1484)
 - Include `valueFileSecrets` property in `helm-operator-crd.yaml`
   [weaveworks/flux#1468](https://github.com/weaveworks/flux/pull/1468)
 - Uniform language highlight on Helm chart README
   [weaveworks/flux#1464](https://github.com/weaveworks/flux/pull/1463)

## 0.4.0 (2018-10-25)

### Bug fixes

 - Made maximum memcache item size configurable, fixes
   `SERVER_ERROR object too large for cache`  errors on large deployments
   [weaveworks/flux#1453](https://github.com/weaveworks/flux/pull/1453)
 - Fixed indentation of `aditionalArgs`
   [weaveworks/flux#1417](https://github.com/weaveworks/flux/pull/1417)

### Improvements

 - Updated Flux to `v1.8.0` and the Helm operator to `0.3.0`
   [weaveworks/flux#1470](https://github.com/weaveworks/flux/pull/1470)
 - Deprecated Flux `--registry-cache-expiry` config flag
   [weaveworks/flux#1470](https://github.com/weaveworks/flux/pull/1470)
 - Added and documented multiple values (s.a. `nodeSelector`,
   `extraEnvs`, `git.timeout`)
   [weaveworks/flux#1469](https://github.com/weaveworks/flux/pull/1469)
   [weaveworks/flux#1446](https://github.com/weaveworks/flux/pull/1446)
   [weaveworks/flux#1416](https://github.com/weaveworks/flux/pull/1416)
 - Made it possible to enable Promotheus annotations
   [weaveworks/flux#1462](https://github.com/weaveworks/flux/pull/1462)

## 0.3.4 (2018-09-28)

### Improvements

 - Updated Flux to `v1.7.1`
   [weaveworks/flux#1405](https://github.com/weaveworks/flux/pull/1405)
 - Custom SSH keys for Flux and Helm operator are now allowed
   [weaveworks/flux#1391](https://github.com/weaveworks/flux/pull/1391)

## 0.3.3 (2018-09-18)

### Improvements

 - Updated Flux to `v1.7.0` and the Helm operator to `v0.2.1`
   [weaveworks/flux#1368](https://github.com/weaveworks/flux/pull/1368)
 - Added memcached verbose option
   [weaveworks/flux#1350](https://github.com/weaveworks/flux/pull/1350)
 - Allow overrides of `.kube/config`
   [weaveworks/flux#1342](https://github.com/weaveworks/flux/pull/1342)
 - Documentation improvements
   [weaveworks/flux#1357](https://github.com/weaveworks/flux/pull/1357)

## 0.3.2 (2018-08-31)

### Improvements

 - Updated Flux to `v1.6.0`
   [weaveworks/flux#1330](https://github.com/weaveworks/flux/pull/1330)
 - Made the Helm operator CRD creation optional
   [weaveworks/flux#1311](https://github.com/weaveworks/flux/pull/1311)

## 0.3.0 (2018-08-24)

### Improvements

 - Updated Helm operator to `v0.2.0`
   [weaveworks/flux#1308](https://github.com/weaveworks/flux/pull/1308)
 - Added Flux git label and registry options
   [weaveworks/flux#1305](https://github.com/weaveworks/flux/pull/1305)
 - Removed `.Values.git.gitPath` value
   [weaveworks/flux#1305](https://github.com/weaveworks/flux/pull/1305)
 - Documented how to use a private Git host
   [weaveworks/flux#1299](https://github.com/weaveworks/flux/pull/1299)
 - Added option to opt-in to logging of release diffs
   [weaveworks/flux#1271](https://github.com/weaveworks/flux/pull/1272)

## 0.2.2 (2018-08-09)

### Bug fixes

 - Fixed indentation of `.Values.ssh.known_hosts`
   [weaveworks/flux#1246](https://github.com/weaveworks/flux/pull/1246)

### Improvements

 - Updated Flux to `v1.5.0`
   [weaveworks/flux#1279](https://github.com/weaveworks/flux/pull/1279)
 - Added openAPIV3Schema validation to Helm CRD
   [weaveworks/flux#1253](https://github.com/weaveworks/flux/pull/1253)
 - Fix markdown typo in README
   [weaveworks/flux#1248](https://github.com/weaveworks/flux/pull/1248)
