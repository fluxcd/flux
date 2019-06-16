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
