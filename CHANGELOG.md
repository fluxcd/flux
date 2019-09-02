## 1.14.2 (2019-09-02)

This is a patch release, with some important fixes to the handling of
HelmRelease resources.

### Fixes

- Correct a problem that prevented automated HelmRelease updates
  [fluxcd/flux#2412][]
- Fix a crash triggered when `helm.fluxcd.io/v1` resources are present
  in the cluster [fluxcd/flux#2404][]

### Enhancements

- Add a flag `--k8s-verbosity`, for controlling Kubernetes client
  logging (formerly, this was left disabled) [fluxcd/flux#2410][]

### Maintenance and documentation

- Rakuten is now listed as a production user [fluxcd/flux#2413][]

### Thanks

Bouquets to @HighwayofLife, @IsNull, @adeleglise, @aliartiza75,
@antonosmond, @bforchhammer, @brunowego, @cartyc, @chainlink,
@cristian-radu, @dholbach, @dranner-bgt, @fshot, @hiddeco, @isen-ng,
@jonohill, @kingdonb, @mflendrich, @mfrister, @mgenov, @raravena80,
@rndstr, @robertgates55, @sklemmer, @smartpcr, @squaremo,
@stefanprodan, @stefansedich, @yellowmegaman, @ysaakpr for
contributions to this release.

[fluxcd/flux#2404]: https://github.com/fluxcd/flux/pull/2404
[fluxcd/flux#2410]: https://github.com/fluxcd/flux/pull/2410
[fluxcd/flux#2412]: https://github.com/fluxcd/flux/pull/2412
[fluxcd/flux#2413]: https://github.com/fluxcd/flux/pull/2413

## 1.14.1 (2019-08-22)

This is a patch release.

### Fixes

- Automated updates of auto detected images in `HelmRelease`
  resources has been fixed
  [fluxcd/flux#2400][]
- `fluxctl install` `--git-paths` option has been replaced by
  `--git-path`, to match the `fluxd` option, the `--git-paths` has
  been deprecated but still works
  [fluxcd/flux#2392][]
- `fluxctl` port forward looks for a pod with one of the labels again,
  instead of stopping when the first label did not return a result
  [fluxcd/flux#2394][]  

### Maintenance and documentation

- Starbucks is now listed as production user (:tada:!)
  [fluxcd/flux#2389][]
- Various fixes to the installation documentation
  fluxcd/flux{[#2384][fluxcd/flux#2384], [#2395][fluxcd/flux#2395]}
- Snap build has been updated to work with Go Modules and Go `1.12.x`
  [fluxcd/flux#2385][]
- Typo fixes in code comments
  [fluxcd/flux#2381][]
  
### Thanks

Thanks @aliartiza75, @ethan-daocloud, @HighwayOfLife, @stefanprodan,
@2opremio, @dhbolach, @mbridgen, @hiddeco for contributing to this
release.
 
[fluxcd/flux#2381]: https://github.com/fluxcd/flux/pull/2381
[fluxcd/flux#2384]: https://github.com/fluxcd/flux/pull/2384
[fluxcd/flux#2385]: https://github.com/fluxcd/flux/pull/2385
[fluxcd/flux#2389]: https://github.com/fluxcd/flux/pull/2389
[fluxcd/flux#2392]: https://github.com/fluxcd/flux/pull/2392
[fluxcd/flux#2394]: https://github.com/fluxcd/flux/pull/2394
[fluxcd/flux#2395]: https://github.com/fluxcd/flux/pull/2395
[fluxcd/flux#2400]: https://github.com/fluxcd/flux/pull/2400

## 1.14.0 (2019-08-21)

This feature release adds a read-only mode to the Flux daemon, adds
support for mapping images in `HelmRelease` resources using YAML dot
notation annotations, eases the deployment of Flux with a new `fluxctl
install` command which generates the required YAML manifests, lots of
documentation improvements, and many more.

### Fixes

- Fetch before branch check to detect upstream changes made after the
  initial clone
  [fluxcd/flux#2371][]

### Enhancements

- With `--git-readonly`, `fluxd` can now sync a git repo without having
  write access to it. In this mode, `fluxd` will not make any commits
  to the repo.
  [fluxcd/flux#1807][]
- Mapping images in `HelmRelease resources` using YAML dot notation
  annotations is now supported
  [fluxcd/flux#2249][]
- `fluxctl` has a new `install` command to ease generating the YAML
  manifests required to deploy Flux
  [fluxcd/flux#2287][]
- Kubectl and Kustomize have been upgraded
  - `kubectl` -> `1.13.8` [fluxcd/flux#2327][]
  - `kustomize` -> `3.1.0` [fluxcd/flux#2299][]
- The annotation domain has been changed to `fluxcd.io`, but backwards
  compatibility with the old (`flux.weave.works`) domain is maintained
  [fluxcd/flux#2219][]
- The number of sorts done by `ListImagesWithOptions` has been reduced
  [fluxcd/flux#2338][]
- `fluxctl` will only look for running `fluxcd` pods while attempting
  to setup a port forward
  [fluxcd/flux#2283][]
- `--registry-poll-interval` has been renamed to `--automation-interval`
  to better reflect what it controls; the interval at which automated
  workloads are checked for updates, and updated.
  [fluxcd/flux#2284][]
- `fluxctl` now has a global `--timeout` flag, which controls how long
  it waits for jobs sent to `fluxd` to complete
  [fluxcd/flux#2056][]

### Maintenance and documentation

- Documentation is now hosted on ReadTheDocs
  [fluxcd/flux#2152][]
- Helm Operator has been removed from the codebase, as it has been moved
  to a dedicated repository (`fluxcd/helm-operator`)
  fluxcd/flux{[#2329][fluxcd/flux#2329], [#2356][fluxcd/flux#2356]}
- Documentation on how to use `fluxctl install` has been added
  [fluxcd/flux#2298][]
- Reference about automated image updates has been added to the
  documentation
  [fluxcd/flux#2369][]
- Documentation has been added on how to deploy Flux with Kustomize
  [fluxcd/flux#2375][]
- CLVR, IBM Cloudant, Omise, Replicated, and Yusofleet are now listed as
  production users (:tada:!)
  fluxcd/flux{[#2331][fluxcd/flux#2331], [#2343][fluxcd/flux#2342], [#2360][fluxcd/flux#2360], [#2373][fluxcd/flux#2373], [#2378][fluxcd/flux#2378]}
- Various changes to the documentation
  fluxcd/flux{[#2306][fluxcd/flux#2306], [#2311][fluxcd/flux#2311], [#2313][fluxcd/flux#2313], [#2314][fluxcd/flux#2314],
    [#2315][fluxcd/flux#2315], [#2332][fluxcd/flux#2332], [#2351][fluxcd/flux#2351], [#2353][fluxcd/flux#2353],
    [#2358][fluxcd/flux#2358], [#2363][fluxcd/flux#2363], [#2364][fluxcd/flux#2364], [#2365][fluxcd/flux#2365],
    [#2367][fluxcd/flux#2367], [#2368][fluxcd/flux#2368], [#2372][fluxcd/flux#2372]}
- Soon-to-be deprecated version script has been removed from the Snapcraft
  build configuration
  [fluxcd/flux#2350][]
- Various typos have been fixed
  fluxcd/flux{[#2348][fluxcd/flux#2348], [#2352][fluxcd/flux#2352], [#2295][fluxcd/flux#2295]}
- Various CI build tweaks (i.a. support preleases containing numbers, Go
  tarball cleanup after installation, Helm chart release changes)
  fluxcd/flux{[#2301][fluxcd/flux#2301], [#2302][fluxcd/flux#2302], [#2312][fluxcd/flux#2312], [#2320][fluxcd/flux#2320], 
    [#2336][fluxcd/flux#2336], [#2349][fluxcd/flux#2349], [#2361][fluxcd/flux#2361]}
- Helm chart repository has been changed to `charts.fluxcd.io`
  fluxcd/flux{[#2337][fluxcd/flux#2337], [#2339][fluxcd/flux#2339], [#2341][fluxcd/flux#2341]}
  
### Thanks

Many thanks for contributions from @2opremio, @AndriiOmelianenko,
@GODBS, @JDavis10213, @MehrCurry, @Sleepy-GH, @adusumillipraveen,
@ainmosni, @alanjcastonguay, @aliartiza75, @autarchprinceps,
@benmathews, @blancsys, @carlosjgp, @cristian-radu, @cristian04,
@davidkarlsen, @dcherman, @demisx, @derrickburns, @dholbach,
@ethan-daocloud, @fred, @gldraphael, @hiddeco, @hlascelles, @ianmiell,
@ilya-spv, @jacobsin, @judewin-alef, @jwenz723, @kaspernissen,
@knackaron, @ksaritek, @larhauga, @laverya, @linuxbsdfreak,
@luxas, @matthewbednarski, @mhumeSF, @mzachh, @nabadger, @obiesmans,
@ogerbron, @onedr0p, @paulmil1, @primeroz, @rhockenbury, @runningman84,
@rytswd, @semyonslepov, @squaremo, @stealthybox, @stefanprodan,
@stefansedich, @suvl, @tjanson, @tomaszkiewicz, @tomcheah, @tschonnie,
@ttarczynski, @willholley, @yellowmegaman, @zcourt.
  
[fluxcd/flux#1807]: https://github.com/fluxcd/flux/pull/1807
[fluxcd/flux#2056]: https://github.com/fluxcd/flux/pull/2056
[fluxcd/flux#2152]: https://github.com/fluxcd/flux/pull/2152
[fluxcd/flux#2219]: https://github.com/fluxcd/flux/pull/2219
[fluxcd/flux#2249]: https://github.com/fluxcd/flux/pull/2249
[fluxcd/flux#2283]: https://github.com/fluxcd/flux/pull/2283
[fluxcd/flux#2284]: https://github.com/fluxcd/flux/pull/2284
[fluxcd/flux#2287]: https://github.com/fluxcd/flux/pull/2287
[fluxcd/flux#2295]: https://github.com/fluxcd/flux/pull/2295
[fluxcd/flux#2298]: https://github.com/fluxcd/flux/pull/2298
[fluxcd/flux#2299]: https://github.com/fluxcd/flux/pull/2299
[fluxcd/flux#2301]: https://github.com/fluxcd/flux/pull/2301
[fluxcd/flux#2302]: https://github.com/fluxcd/flux/pull/2302
[fluxcd/flux#2306]: https://github.com/fluxcd/flux/pull/2306
[fluxcd/flux#2311]: https://github.com/fluxcd/flux/pull/2311
[fluxcd/flux#2312]: https://github.com/fluxcd/flux/pull/2312
[fluxcd/flux#2313]: https://github.com/fluxcd/flux/pull/2313
[fluxcd/flux#2314]: https://github.com/fluxcd/flux/pull/2314
[fluxcd/flux#2315]: https://github.com/fluxcd/flux/pull/2315
[fluxcd/flux#2320]: https://github.com/fluxcd/flux/pull/2320
[fluxcd/flux#2327]: https://github.com/fluxcd/flux/pull/2327
[fluxcd/flux#2329]: https://github.com/fluxcd/flux/pull/2329
[fluxcd/flux#2331]: https://github.com/fluxcd/flux/pull/2331
[fluxcd/flux#2332]: https://github.com/fluxcd/flux/pull/2332
[fluxcd/flux#2336]: https://github.com/fluxcd/flux/pull/2336
[fluxcd/flux#2337]: https://github.com/fluxcd/flux/pull/2337
[fluxcd/flux#2338]: https://github.com/fluxcd/flux/pull/2338
[fluxcd/flux#2339]: https://github.com/fluxcd/flux/pull/2339
[fluxcd/flux#2341]: https://github.com/fluxcd/flux/pull/2341
[fluxcd/flux#2342]: https://github.com/fluxcd/flux/pull/2342
[fluxcd/flux#2348]: https://github.com/fluxcd/flux/pull/2348
[fluxcd/flux#2349]: https://github.com/fluxcd/flux/pull/2349
[fluxcd/flux#2350]: https://github.com/fluxcd/flux/pull/2350
[fluxcd/flux#2351]: https://github.com/fluxcd/flux/pull/2351
[fluxcd/flux#2352]: https://github.com/fluxcd/flux/pull/2352
[fluxcd/flux#2353]: https://github.com/fluxcd/flux/pull/2353
[fluxcd/flux#2356]: https://github.com/fluxcd/flux/pull/2356
[fluxcd/flux#2358]: https://github.com/fluxcd/flux/pull/2358
[fluxcd/flux#2360]: https://github.com/fluxcd/flux/pull/2360
[fluxcd/flux#2361]: https://github.com/fluxcd/flux/pull/2361
[fluxcd/flux#2363]: https://github.com/fluxcd/flux/pull/2363
[fluxcd/flux#2364]: https://github.com/fluxcd/flux/pull/2364
[fluxcd/flux#2365]: https://github.com/fluxcd/flux/pull/2365
[fluxcd/flux#2367]: https://github.com/fluxcd/flux/pull/2367
[fluxcd/flux#2368]: https://github.com/fluxcd/flux/pull/2368
[fluxcd/flux#2369]: https://github.com/fluxcd/flux/pull/2369
[fluxcd/flux#2371]: https://github.com/fluxcd/flux/pull/2371
[fluxcd/flux#2372]: https://github.com/fluxcd/flux/pull/2372
[fluxcd/flux#2373]: https://github.com/fluxcd/flux/pull/2373
[fluxcd/flux#2375]: https://github.com/fluxcd/flux/pull/2375
[fluxcd/flux#2378]: https://github.com/fluxcd/flux/pull/2378

## 1.13.3 (2019-07-25)

This is a patch release, mostly concerned with adapting documentation
to Flux's new home in https://github.com/fluxcd/ and the [CNCF
sandbox](https://www.cncf.io/sandbox-projects/).

### Fixes

- Correct the name of the `--registry-require` argument mentioned in a
  log message [fluxcd/flux#2256][]
- Parse Docker credentials that have a host and port, but not a scheme
  [fluxcd/flux#2248][]

### Maintenance and documentation

- Change references to weaveworks/flux to fluxcd/flux
  [fluxcd/flux#2240][], [fluxcd/flux#2244][], [fluxcd/flux#2257][],
  [fluxcd/flux#2271][]
- Add Walmart to production users (:tada:!) [fluxcd/flux#2268][]
- Mention the multi-tenancy tutorial in the README
  [fluxcd/flux#2286][]
- Fix the filename given in the `.flux.yaml` (manifest generation)
  docs [fluxcd/flux#2270][]
- Run credentials tests in parallel, without sleeping
  [fluxcd/flux#2254][]
- Correct the Prometheus annotations given in examples
  [fluxcd/flux#2278][]

### Thanks

Thanks to the following for contributions since the last release:
@2opremio, @aaron-trout, @adusumillipraveen, @alexhumphreys,
@aliartiza75, @ariep, @binjheBenjamin, @bricef, @caniszczyk,
@carlosjgp, @carlpett, @chriscorn-takt, @cloudoutloud, @derrickburns,
@dholbach, @fnmeissner, @gled4er, @hiddeco, @jmtrusona, @jowparks,
@jpellizzari, @ksaritek, @ktsakalozos, @mar1n3r0, @mzachh, @primeroz,
@squaremo, @stefanprodan, @sureshamk, @vyckou, @ybaruchel, @zoni.

[fluxcd/flux#2240]: https://github.com/fluxcd/flux/pull/2240
[fluxcd/flux#2244]: https://github.com/fluxcd/flux/pull/2244
[fluxcd/flux#2248]: https://github.com/fluxcd/flux/pull/2248
[fluxcd/flux#2254]: https://github.com/fluxcd/flux/pull/2254
[fluxcd/flux#2256]: https://github.com/fluxcd/flux/pull/2256
[fluxcd/flux#2257]: https://github.com/fluxcd/flux/pull/2257
[fluxcd/flux#2268]: https://github.com/fluxcd/flux/pull/2268
[fluxcd/flux#2270]: https://github.com/fluxcd/flux/pull/2270
[fluxcd/flux#2271]: https://github.com/fluxcd/flux/pull/2271
[fluxcd/flux#2278]: https://github.com/fluxcd/flux/pull/2278
[fluxcd/flux#2286]: https://github.com/fluxcd/flux/pull/2286

## 1.13.2 (2019-07-10)

This is a patch release, including a fix for [problems with using image
labels as timestamps][weaveworks/flux#2176].

### Fixes

- Because image labels are inherited from base images, fluxd cannot
  indiscriminately use labels to determine the image created date. You
  must now explicitly allow that behaviour with the argument
  `--registry-use-labels` [weaveworks/flux#2176][]
- Image timestamps can be missing (or zero) if ordering them by semver
  version rather than timestamp [weaveworks/flux#2175][]
- Environment variables needed by the Google Cloud SDK helper are now
  propagated to git [weaveworks/flux#2222][]

### Maintenance and documentation

- Image builds are pushed to both weaveworks/ and fluxcd/ orgs on
  DockerHub, in preparation for the project moving organisations
  [weaveworks/flux#2213][]
- Calculate Go dependencies more efficiently during the build
  [weaveworks/flux#2207][]
- Refactor to remove a spurious top-level package
  [weaveworks/flux#2201][]
- Update the version of Kubernetes-in-Docker used in end-to-end test,
  to v0.4.0 [weaveworks/flux#2202][]
- Bump the Ubuntu version used in CI [weaveworks/flux#2195][]

### Thanks

Thanks go to the following for contributions: @2opremio, @4c74356b41,
@ArchiFleKs, @adrian, @alanjcastonguay, @alexanderbuhler,
@alexhumphreys, @bobbytables, @derrickburns, @dholbach, @dlespiau,
@gaffneyd4, @hiddeco, @hkalsi, @hlascelles, @jaksonwkr, @jblunck,
@jwenz723, @linuxbsdfreak, @luxas, @mpashka, @nlamot, @semyonslepov,
@squaremo, @stefanprodan, @tegamckinney, @ysaakpr.

[weaveworks/flux#2175]: https://github.com/weaveworks/flux/pull/2175
[weaveworks/flux#2176]: https://github.com/weaveworks/flux/pull/2176
[weaveworks/flux#2195]: https://github.com/weaveworks/flux/pull/2195
[weaveworks/flux#2201]: https://github.com/weaveworks/flux/pull/2201
[weaveworks/flux#2202]: https://github.com/weaveworks/flux/pull/2202
[weaveworks/flux#2207]: https://github.com/weaveworks/flux/pull/2207
[weaveworks/flux#2213]: https://github.com/weaveworks/flux/pull/2213
[weaveworks/flux#2222]: https://github.com/weaveworks/flux/pull/2222

## 1.13.1 (2019-06-27)

This is a patch release.

### Fixes

- Use a context with a timeout for every request that comes through
  the upstream connection, so they may be abandoned if taking too long [weaveworks/flux#2171][]
- Initialise the high-water mark once, so it doesn't get continually
  reset and cause notification noise [weaveworks/flux#2177][]
- Force tag updates when making local clones, to account for changes
  in git 2.20 [weaveworks/flux#2184][]

### Thanks

Cheers to the following people for their contributions: @2opremio,
@J-Lou, @aarnaud, @adrian, @airmap-madison, @alanjcastonguay,
@arsiesys, @atbe-crowe, @azazel75, @bia, @carlosjgp, @chriscorn-takt,
@cristian-radu, @davidkarlsen, @derrickburns, @dholbach, @dlespiau,
@errordeveloper, @ewoutp, @hiddeco, @humayunjamal, @isen-ng,
@judewin-alef, @kevinm444, @muhlba91, @roaddemon, @runningman84,
@squaremo, @starkers, @stefanprodan, @sukrit007, @willholley.

[weaveworks/flux#2171]: https://github.com/weaveworks/flux/pull/2171
[weaveworks/flux#2177]: https://github.com/weaveworks/flux/pull/2177
[weaveworks/flux#2184]: https://github.com/weaveworks/flux/pull/2184

## 1.13.0 (2019-06-13)

This feature release contains an experimental feature for [generating
manifests from the sources in git][manifest-generation-docs] and
completes the support for [GPG signatures][gpg-docs].

### Fixes

- Use openssh-client rather than openssh in container image
  [weaveworks/flux#2142][]
- Cope when filenames from git start or end with spaces
  [weaveworks/flux#2117][]
- Ignore `metrics` API group, known to be problematic
  [weaveworks/flux#2096][]
- Remove a possible deadlock from code calling `git`
  [weaveworks/flux#2086][]

### Enhancements

- When `--manifest-generation` is set, look for `.flux.yaml` files in
  the git repo and generate manifests according to the instructions
  therein (see [the docs][manifest-generation-docs])
  [weaveworks/flux#1848][]
- Verify GPG signatures on commits (when `--git-verify-signatures` is
  set; see [the docs][gpg-docs]) [weaveworks/flux#1791][]
- Make the log format configurable (specifically to admit JSON
  logging) [weaveworks/flux#2138][]
- Log when a requested workload is not of a kind known to fluxd
  [weaveworks/flux#2097][]
- Get image build time from OCI labels, if present
  [weaveworks/flux#1992][], [weaveworks/flux#2084][]
- A new flag `--garbage-collection-dry-run` will report what _would_
  be deleted by garbage collection in the log, without deleting it
  [weaveworks/flux#2063][]

### Maintenance and documentation

- Let fluxd be run outside a cluster, for development convenience
  [weaveworks/flux#2140][]
- Documentation edits [weaveworks/flux#2134][], [weaveworks/flux#2109][]
- Improve some tests [weaveworks/flux#2111][], [weaveworks/flux#2110][],
  [weaveworks/flux#2085][], [weaveworks/flux#2090][]
- Give the memcached pod a security context [weaveworks/flux#2125][]
- Move to `go mod`ules and abandon `go dep` [weaveworks/flux#2083][],
  [weaveworks/flux#2127][], [weaveworks/flux#2094][]
- Give an example of DNS settings in the example deployment
  [weaveworks/flux#2116][]
- Document how to get the fluxctl `snap` [weaveworks/flux#1966][],
  [weaveworks/flux#2108][]
- Give more guidance on how to contribute to Flux
  [weaveworks/flux#2104][]
- Speed CI builds up by using CircleCI caching [weaveworks/flux#2078][]

### Thanks

Many thanks for contributions from @2opremio, @AndriiOmelianenko,
@ArchiFleKs, @RGPosadas, @RoryShively, @alanjcastonguay, @amstee,
@arturo-c, @azazel75, @billimek, @brezerk, @bzon, @derrickburns,
@dholbach, @dminca, @dmitri-lerko, @guzmo, @hiddeco, @imrtfm,
@jan-schumacher, @jp83, @jpds, @kennethredler, @leoblanc,
@marcelonaso, @marcossv9, @marklcg, @michaelgeorgeattard, @mr-karan,
@nabadger, @ncabatoff, @primeroz, @rdubya16, @rjanovski,
@rkouyoumjian, @rndstr, @runningman84, @squaremo, @stefanprodan,
@stefansedich, @suvl, @tckb, @timja, @vovkanaz, @willholley.

[weaveworks/flux#1791]: https://github.com/weaveworks/flux/pull/1791
[weaveworks/flux#1848]: https://github.com/weaveworks/flux/pull/1848
[weaveworks/flux#1966]: https://github.com/weaveworks/flux/pull/1966
[weaveworks/flux#1992]: https://github.com/weaveworks/flux/pull/1992
[weaveworks/flux#2063]: https://github.com/weaveworks/flux/pull/2063
[weaveworks/flux#2078]: https://github.com/weaveworks/flux/pull/2078
[weaveworks/flux#2083]: https://github.com/weaveworks/flux/pull/2083
[weaveworks/flux#2084]: https://github.com/weaveworks/flux/pull/2084
[weaveworks/flux#2085]: https://github.com/weaveworks/flux/pull/2085
[weaveworks/flux#2086]: https://github.com/weaveworks/flux/pull/2086
[weaveworks/flux#2090]: https://github.com/weaveworks/flux/pull/2090
[weaveworks/flux#2094]: https://github.com/weaveworks/flux/pull/2094
[weaveworks/flux#2096]: https://github.com/weaveworks/flux/pull/2096
[weaveworks/flux#2097]: https://github.com/weaveworks/flux/pull/2097
[weaveworks/flux#2104]: https://github.com/weaveworks/flux/pull/2104
[weaveworks/flux#2108]: https://github.com/weaveworks/flux/pull/2108
[weaveworks/flux#2109]: https://github.com/weaveworks/flux/pull/2109
[weaveworks/flux#2110]: https://github.com/weaveworks/flux/pull/2110
[weaveworks/flux#2111]: https://github.com/weaveworks/flux/pull/2111
[weaveworks/flux#2116]: https://github.com/weaveworks/flux/pull/2116
[weaveworks/flux#2117]: https://github.com/weaveworks/flux/pull/2117
[weaveworks/flux#2125]: https://github.com/weaveworks/flux/pull/2125
[weaveworks/flux#2127]: https://github.com/weaveworks/flux/pull/2127
[weaveworks/flux#2134]: https://github.com/weaveworks/flux/pull/2134
[weaveworks/flux#2138]: https://github.com/weaveworks/flux/pull/2138
[weaveworks/flux#2140]: https://github.com/weaveworks/flux/pull/2140
[weaveworks/flux#2142]: https://github.com/weaveworks/flux/pull/2142
[manifest-generation-docs]: https://github.com/weaveworks/flux/blob/master/site/fluxyaml-config-files.md
[gpg-docs]: https://github.com/weaveworks/flux/blob/master/site/git-gpg.md

## 1.12.3 (2019-05-22)

This is a patch release.

### Fixes

- Show tag image for workload in list-images
  [weaveworks/flux#2024][]
- Log warning when not applying resource by namespace
  [weaveworks/flux#2034][]
- Always list the status of a workload in `fluxctl`
  [weaveworks/flux#2035][]
- Ensure Flux installs gnutls >=3.6.7, to resolve security scan issues
  [weaveworks/flux#2044][]
- Rename controller to workload in `fluxctl release`
  [weaveworks/flux#2048][]
- Give full output of git command on errors
  [weaveworks/flux#2054][]

### Maintenance and documentation

- Warn about Flux only supporting YAML and not JSON
  [weaveworks/flux#2010][]
- Fix and refactor end-to-end tests
  [weaveworks/flux#2050][] [weaveworks/flux#2058][]

### Thanks

Thanks to @2opremio, @hiddeco, @squaremo and @xtellurian for contributions.

[weaveworks/flux#2010]: https://github.com/weaveworks/flux/pull/2010
[weaveworks/flux#2024]: https://github.com/weaveworks/flux/pull/2024
[weaveworks/flux#2034]: https://github.com/weaveworks/flux/pull/2034
[weaveworks/flux#2035]: https://github.com/weaveworks/flux/pull/2035
[weaveworks/flux#2044]: https://github.com/weaveworks/flux/pull/2044
[weaveworks/flux#2048]: https://github.com/weaveworks/flux/pull/2048
[weaveworks/flux#2050]: https://github.com/weaveworks/flux/pull/2050
[weaveworks/flux#2054]: https://github.com/weaveworks/flux/pull/2054
[weaveworks/flux#2058]: https://github.com/weaveworks/flux/pull/2058

## 1.12.2 (2019-05-08)

This is a patch release.

### Fixes

- Fix error shadowing when parsing YAML manifests
  [weaveworks/flux#1994][]
- Fix 'workspace' -> 'workload' typo in deprecated controller flag
  [weaveworks/flux#1987][] [weaveworks/flux#1996][]
- Improve internal Kubernetes error logging, by removing the duplicate
  timestamp and providing a full path to the Kubernetes file emitting
  the error
  [weaveworks/flux#2000][]
- Improve `fluxctl` auto portforward connection error, by better
  guiding the user about what could be wrong
  [weaveworks/flux#2001][]
- Ignore discovery errors for metrics resources, to prevent syncs from
  failing when the metrics API is misconfigured
  [weaveworks/flux#2009][]
- Fix `(Flux)HelmRelease` cluster lookups, before this change, the
  same resource ID would be reported for all `HelmRelease`s with e.g.
  `fluctl list-workloads`
  [weaveworks/flux#2018][]
  

### Maintenance and documentation

- Replace deprecated `--controller` flag in documentation with
  `--workload`
  [weaveworks/flux#1985][]
- Update `MAINTAINERS` and include email addresses
  [weaveworks/flux#1995][]

### Thanks

Thanks to @2opremio, @cdenneen, @hiddeco, @jan-schumacher, @squaremo,
@stefanprodan for contributions.

[weaveworks/flux#1985]: https://github.com/weaveworks/flux/pull/1985
[weaveworks/flux#1987]: https://github.com/weaveworks/flux/pull/1987
[weaveworks/flux#1994]: https://github.com/weaveworks/flux/pull/1994
[weaveworks/flux#1995]: https://github.com/weaveworks/flux/pull/1995
[weaveworks/flux#1996]: https://github.com/weaveworks/flux/pull/1996
[weaveworks/flux#2000]: https://github.com/weaveworks/flux/pull/2000
[weaveworks/flux#2001]: https://github.com/weaveworks/flux/pull/2001
[weaveworks/flux#2009]: https://github.com/weaveworks/flux/pull/2009
[weaveworks/flux#2018]: https://github.com/weaveworks/flux/pull/2018

## 1.12.1 (2019-04-25)

This is a patch release.

### Fixes

- Be more tolerant of image manifests being missing in the registry,
  when we don't need them [weaveworks/flux#1916][]
- Give image registry fetches a timeout, so the image metadata DB
  doesn't get stuck [weaveworks/flux#1970][]
- Allow insecure host arguments to exclude the port
  [weaveworks/flux#1967][]
- Make sure client-go logs to stderr [weaveworks/flux#1945][]
- Cope gracefully when custom API resources are not present in the
  cluster or in git (and therefore we cannot determine how a custom
  resource is scoped) [weaveworks/flux#1943][]
- Warn when the configured branch does not exist in git, and use the
  configured branch to check writablility [weaveworks/flux#1937][]
- Deal with YAML document end markers [weaveworks/flux#1931][],
  [weaveworks/flux#1973][]

### Maintenance and documentation

- Add some known production users to the README
  [weaveworks/flux#1958][], [weaveworks/flux#1946][],
  [weaveworks/flux#1932][]
- Move images to DockerHub and have a separate pre-releases image repo
  [weaveworks/flux#1949][], [weaveworks/flux#1956][]
- Support `arm` and `arm64` builds [weaveworks/flux#1950][]
- Refactor the core image metadata fetching func
  [weaveworks/flux#1935][]
- Update client-go to v1.11 [weaveworks/flux#1929][]
- Retry keyscan when building images, to mitigate for occasional
  timeouts [weaveworks/flux#1971][]
- Give the GitHub repo an issue template for bug reports
  [weaveworks/flux#1968][]

### Thanks

Thanks to @2opremio, @UnwashedMeme, @alexanderbuhler, @aronne,
@arturo-c, @autarchprinceps, @benhartley, @brantb, @brezerk,
@dholbach, @dlespiau, @dvelitchkov, @dwightbiddle-ef, @gtseres,
@hiddeco, @hpurmann, @ingshtrom, @isen-ng, @jimangel, @jpds,
@kingdonb, @koustubh25, @koustubhg, @michaelfig, @moltar, @nabadger,
@primeroz, @rdubya16, @squaremo, @stealthybox, @stefanprodan, @tycoles
for contributions.

[weaveworks/flux#1916]: https://github.com/weaveworks/flux/pull/1916
[weaveworks/flux#1929]: https://github.com/weaveworks/flux/pull/1929
[weaveworks/flux#1931]: https://github.com/weaveworks/flux/pull/1931
[weaveworks/flux#1932]: https://github.com/weaveworks/flux/pull/1932
[weaveworks/flux#1935]: https://github.com/weaveworks/flux/pull/1935
[weaveworks/flux#1937]: https://github.com/weaveworks/flux/pull/1937
[weaveworks/flux#1943]: https://github.com/weaveworks/flux/pull/1943
[weaveworks/flux#1945]: https://github.com/weaveworks/flux/pull/1945
[weaveworks/flux#1946]: https://github.com/weaveworks/flux/pull/1946
[weaveworks/flux#1949]: https://github.com/weaveworks/flux/pull/1949
[weaveworks/flux#1950]: https://github.com/weaveworks/flux/pull/1950
[weaveworks/flux#1956]: https://github.com/weaveworks/flux/pull/1956
[weaveworks/flux#1958]: https://github.com/weaveworks/flux/pull/1958
[weaveworks/flux#1967]: https://github.com/weaveworks/flux/pull/1967
[weaveworks/flux#1968]: https://github.com/weaveworks/flux/pull/1968
[weaveworks/flux#1970]: https://github.com/weaveworks/flux/pull/1970
[weaveworks/flux#1971]: https://github.com/weaveworks/flux/pull/1971
[weaveworks/flux#1973]: https://github.com/weaveworks/flux/pull/1973

## 1.12.0 (2019-04-11)

This release renames some fluxctl commands and arguments while
deprecating others, to better follow Kubernetes terminology. In
particular, it drops the term "controller" in favour of "workload";
e.g., instead of

    fluxctl list-controllers --controller=...

there is now

    fluxctl list-workloads --workload=...

The old commands are deprecated but still available for now.

It also extends the namespace restriction flag
(`--k8s-allow-namespace`, with a deprecated alias
`--k8s-namespace-whitelist`) to cover all operations, including
syncing; previously, it covered only query operations e.g.,
`list-images` etc..

### Fixes

- Periodically refresh memcached addresses, to recover from DNS
  outages [weaveworks/flux#1913][]
- Properly apply `fluxctl policy --tag-all` when a manifest does not
  have a namespace [weaveworks/flux#1901][]
- Support newer git versions (>=2.21) [weaveworks/flux#1884][]
- Avoid errors arising from ambiguous git refs
  [weaveworks/flux#1875][] and [weaveworks/flux#1829][]
- Reload the API definitions periodically, to account for the API
  server being unavailable when starting [weaveworks/flux#1859][]
- Admit `<cluster>` when parsing resource IDs, since it's now used to
  mark cluster-scoped resources [weaveworks/flux#1851][]
- Better recognise and tolerate when Kubernetes API errors mean "not
  accessible" [weaveworks/flux#1840][] and [weaveworks/flux#1832][],
  and stop the Kubernetes client from needlessly logging them
  [weaveworks/flux#1837][]

### Improvements

- Use "workload" as the term for resources that specify pods to run,
  in `fluxctl` commands and wherever else it is needed
  [weaveworks/flux#1777][]
- Make `regex` an alias for `regexp` in tag filters
  [weaveworks/flux#1915][]
- Be more sparing when logging AWS detection failures; add flag for
  requiring AWS authentication; observe ECR restrictions on region and
  account regardless of AWS detection [weaveworks/flux#1863][]
- Treat all `*List` (e.g., `DeploymentList`) resources as lists
  [weaveworks/flux#1883][]
- Add host key for legacy VSTS (now Azure DevOps)
  [weaveworks/flux#1870][]
- Extend namespace restriction to all operations, and change the name
  of the flag to `--k8s-allow-namespace` [weaveworks/flux#1668][]
- Avoid updating images when there is no record for the current image
  [weaveworks/flux#1831][]
- Include the file name in the error when kubeyaml fails to update a
  manifest [weaveworks/flux#1815][]

### Maintenance and documentation

- Avoid creating a cached image when host key verification fails while
  building [weaveworks/flux#1908][]
- Separate "Get started" instructions for fluxd vs. fluxd with the
  Helm operator [weaveworks/flux#1902][], [weaveworks/flux#1912][]
- Add an end-to-end smoke test to run in CI [weaveworks/flux#1800][]
- Make git tracing report more output [weaveworks/flux#1844][]
- Fix flaky API discovery test [weaveworks/flux#1849][]

### Thanks

Many thanks to @2opremio, @AmberAttebery, @alanjcastonguay,
@alexanderbuhler, @arturo-c, @benhartley, @cruisehall, @dholbach,
@dimitropoulos, @hiddeco, @hlascelles, @ipedrazas, @jrryjcksn,
@marchmallow, @mazzy89, @mulcahys, @nabadger, @pmquang,
@southbanksoftwaredeveloper, @squaremo, @srueg, @stefanprodan,
@stevenpall, @stillinbeta, @swade1987, @timfpark, @vanderstack for
contributions.

[weaveworks/flux#1913]: https://github.com/weaveworks/flux/pull/1913
[weaveworks/flux#1912]: https://github.com/weaveworks/flux/pull/1912
[weaveworks/flux#1901]: https://github.com/weaveworks/flux/pull/1901
[weaveworks/flux#1884]: https://github.com/weaveworks/flux/pull/1884
[weaveworks/flux#1875]: https://github.com/weaveworks/flux/pull/1875
[weaveworks/flux#1829]: https://github.com/weaveworks/flux/pull/1829
[weaveworks/flux#1859]: https://github.com/weaveworks/flux/pull/1859
[weaveworks/flux#1851]: https://github.com/weaveworks/flux/pull/1851
[weaveworks/flux#1840]: https://github.com/weaveworks/flux/pull/1840
[weaveworks/flux#1832]: https://github.com/weaveworks/flux/pull/1832
[weaveworks/flux#1837]: https://github.com/weaveworks/flux/pull/1837
[weaveworks/flux#1777]: https://github.com/weaveworks/flux/pull/1777
[weaveworks/flux#1915]: https://github.com/weaveworks/flux/pull/1915
[weaveworks/flux#1863]: https://github.com/weaveworks/flux/pull/1863
[weaveworks/flux#1883]: https://github.com/weaveworks/flux/pull/1883
[weaveworks/flux#1870]: https://github.com/weaveworks/flux/pull/1870
[weaveworks/flux#1668]: https://github.com/weaveworks/flux/pull/1668
[weaveworks/flux#1831]: https://github.com/weaveworks/flux/pull/1831
[weaveworks/flux#1815]: https://github.com/weaveworks/flux/pull/1815
[weaveworks/flux#1908]: https://github.com/weaveworks/flux/pull/1908
[weaveworks/flux#1902]: https://github.com/weaveworks/flux/pull/1902
[weaveworks/flux#1912]: https://github.com/weaveworks/flux/pull/1912
[weaveworks/flux#1800]: https://github.com/weaveworks/flux/pull/1800
[weaveworks/flux#1844]: https://github.com/weaveworks/flux/pull/1844
[weaveworks/flux#1849]: https://github.com/weaveworks/flux/pull/1849

## 1.11.1 (2019-04-01)

This is a bugfix release, fixing a regression introduced in 1.11.0 which caused 
syncs to fail when adding a CRD and instance(s) from that CRD at the same time.

### Fixes

- Obtain scope of CRD instances from its manifest as a fallback
  [weaveworks/flux#1876][#1876]
  
[#1876]: https://github.com/weaveworks/flux/pull/1876

## 1.11.0 (2019-03-13)

This release comes with experimental garbage collection and Git commit signing:

1. Experimental garbage collection of cluster resources. When providing the
   `--sync-garbage-collection` flag, cluster resources no longer existing in Git 
   will be removed. Read the [garbage collection documentation](site/garbagecollection.md) 
   for further details.

2. [GPG](https://en.wikipedia.org/wiki/GNU_Privacy_Guard)
   [Git commit signing](https://git-scm.com/docs/git-commit#Documentation/git-commit.txt--Sltkeyidgt),
   when providing `--git-signing-key` flag. GPG keys can be imported with
   `--git-gpg-key-import`. By default Flux will import to and use the keys
   in `~/.gnupg`. This path can be overridden by setting the `GNUPGHOME` environment
   variable.

   Commit signature verification is in the works and will be released shortly.

### Fixes

- Wait for shutdown before returning from `main()`
  [weaveworks/flux#1789][#1789]
- Make `fluxctl list-images` adhere to namespace filter
  [weaveworks/flux#1763][#1763]
- Take ignore policy into account when working with automated resources
  [weaveworks/flux#1749][#1749]

### Improvements

- Delete resources no longer in git
  [weaveworks/flux#1442][#1442]
  [weaveworks/flux#1798][#1798]
  [weaveworks/flux#1806][#1806]
- Git commit signing
  [weaveworks/flux#1394][#1394]
- Apply user defined Git timeout on all operations
  [weaveworks/flux#1767][#1767]

### Maintenance and documentation

- Bump Alpine version from v3.6 to v3.9
  [weaveworks/flux#1801][#1801]
- Increase memcached memory defaults
  [weaveworks/flux#1780][#1780]
- Update developing docs to remind to `make test`
  [weaveworks/flux#1796][#1796]
- Fix Github link
  [weaveworks/flux#1795][#1795]
- Improve Docs (focusing on local development)
  [weaveworks/flux#1771][#1771]
- Increase timeouts in daemon_test.go
  [weaveworks/flux#1779][#1779]
- Rename resource method `Policy()` to `Policies()`
  [weaveworks/flux#1775][#1775]
- Improve testing in local environments other than linux-amd64
  [weaveworks/flux#1765][#1765]
- Re-flow sections to order by importance
  [weaveworks/flux#1754][#1754]
- Document flux-dev mailing list
  [weaveworks/flux#1755][#1755]
- Updates Docs (wording, typos, formatting)
  [weaveworks/flux#1753][#1753]
- Document source of Azure SSH host key
  [weaveworks/flux#1751][#1751]

### Thanks

Lots of thanks to @2opremio, @Timer, @bboreham, @dholbach, @dimitropoulos,
@hiddeco, @scjudd, @squaremo and @stefanprodan  for their contributions to
this release.

[#1394]: https://github.com/weaveworks/flux/pull/1394
[#1442]: https://github.com/weaveworks/flux/pull/1442
[#1749]: https://github.com/weaveworks/flux/pull/1749
[#1751]: https://github.com/weaveworks/flux/pull/1751
[#1753]: https://github.com/weaveworks/flux/pull/1753
[#1754]: https://github.com/weaveworks/flux/pull/1754
[#1755]: https://github.com/weaveworks/flux/pull/1755
[#1763]: https://github.com/weaveworks/flux/pull/1763
[#1765]: https://github.com/weaveworks/flux/pull/1765
[#1767]: https://github.com/weaveworks/flux/pull/1767
[#1771]: https://github.com/weaveworks/flux/pull/1771
[#1775]: https://github.com/weaveworks/flux/pull/1775
[#1779]: https://github.com/weaveworks/flux/pull/1779
[#1780]: https://github.com/weaveworks/flux/pull/1780
[#1789]: https://github.com/weaveworks/flux/pull/1789
[#1795]: https://github.com/weaveworks/flux/pull/1795
[#1796]: https://github.com/weaveworks/flux/pull/1796
[#1798]: https://github.com/weaveworks/flux/pull/1798
[#1801]: https://github.com/weaveworks/flux/pull/1801
[#1806]: https://github.com/weaveworks/flux/pull/1806

## 1.10.1 (2019-02-13)

This release provides a deeper integration with Azure (DevOps Git hosts
and ACR) and allows configuring how `fluxctl` finds `fluxd` (useful for
clusters with multiple fluxd installations).

### Improvements

- Support Azure DevOps Git hosts
  [weaveworks/flux#1729][#1729]
  [weaveworks/flux#1731][#1731]
- Use AKS credentials for ACR
  [weaveworks/flux#1694][#1694]
- Make port forward label selector configurable
  [weaveworks/flux#1727][#1727]

### Thanks

Lots of thanks to @alanjcastonguay, @hiddeco, and @sarath-p for their
contributions to this release.

[#1694]: https://github.com/weaveworks/flux/pull/1694
[#1727]: https://github.com/weaveworks/flux/pull/1727
[#1729]: https://github.com/weaveworks/flux/pull/1729
[#1731]: https://github.com/weaveworks/flux/pull/1731

## 1.10.0 (2019-02-07)

This release adds the `--registry-exclude-image` flag for excluding
images from scanning, allows for registries with self-signed
certificates, and fixes several bugs.

### Fixes

- Bumped `justinbarrick/go-k8s-portforward` to `1.0.2` to correctly
  handle multiple paths in the `KUBECONFIG` env variable
  [weaveworks/flux#1658][#1658]
- Improved handling of registry challenge requests (preventing memory
  leaks) [weaveworks/flux#1672][#1672]
- Altered merging strategy for image credentials, which previously
  could lead to Flux trying to fetch image details with credentials
  from a different workload [weaveworks/flux#1702][#1702]

### Improvements

- Allow (potentially all) images to be excluded from scanning
  [weaveworks/flux#1659][#1659]
- `--registry-insecure-host` now first tries to skip TLS host
  host verification before falling back to HTTP, allowing registries
  with self-signed certificates [weaveworks/flux#1526][#1526]
- Allow `HOME` env variable when invoking Git which allows for mounting
  a config file under `$HOME/config/git` [weaveworks/flux#1644][#1644]
- Several documentation improvements and clarifications
  weaveworks/flux{[#1656], [#1675], [#1681]}
- Removed last traces of `linting` [weaveworks/flux#1673][#1673]
- Warn users about external changes in sync tag
  [weaveworks/flux#1695][#1695]

### Thanks

Lots of thanks to @2opremio, @alanjcastonguay, @bheesham, @brantb,
@dananichev, @dholbach, @dmarkey, @hiddeco, @ncabatoff, @rade,
@squaremo, @switchboardOp, @stefanprodan and @Timer for their
contributions to this release, and anyone I've missed while writing
this note.

[#1526]: https://github.com/weaveworks/flux/pull/1526
[#1644]: https://github.com/weaveworks/flux/pull/1644
[#1656]: https://github.com/weaveworks/flux/pull/1656
[#1658]: https://github.com/weaveworks/flux/pull/1658
[#1659]: https://github.com/weaveworks/flux/pull/1659
[#1672]: https://github.com/weaveworks/flux/pull/1672
[#1673]: https://github.com/weaveworks/flux/pull/1673
[#1675]: https://github.com/weaveworks/flux/pull/1675
[#1681]: https://github.com/weaveworks/flux/pull/1681
[#1695]: https://github.com/weaveworks/flux/pull/1695
[#1702]: https://github.com/weaveworks/flux/pull/1702

## 1.9.0 (2019-01-09)

This release adds native support for ECR (Amazon Elastic Container
Registry) authentication.

### Fixes

- Make sure a `/etc/hosts` mounted into the fluxd container is
  respected [weaveworks/flux#1630][#1630]
- Proceed more gracefully when RBAC rules restrict access
  [weaveworks/flux#1620][#1620]
- Show more contextual information when `fluxctl` fails
  [weaveworks/flux#1615][#1615]

### Improvements

- Authenticate to ECR using a token from AWS IAM, when possible
  [weaveworks/flux#1619][#1619]
- Make it possible, and the default for new deployments, to configure
  a ClusterIP for memcached (previously it was only possible to use
  DNS service discovery) [weaveworks/flux#1618][#1618]

## Thanks

This release was made possible by welcome contributions from
@2opremio, @agcooke, @cazzoo, @davidkarlsen, @dholbach, @dmarkey,
@donifer, @ericbarch, @errordeveloper, @florianrusch, @gellweiler,
@hiddeco, @isindir, @k, @marcincuber, @markbenschop, @Morriz, @rndstr,
@roffe, @runningman84, @shahbour, @squaremo, @srueg, @stefanprodan,
@stephenmoloney, @switchboardOp, @tobru, @tux-00, @u-phoria,
@Viji-Sarathy-Bose.

[#1615]: https://github.com/weaveworks/flux/pull/1615
[#1618]: https://github.com/weaveworks/flux/pull/1618
[#1619]: https://github.com/weaveworks/flux/pull/1619
[#1620]: https://github.com/weaveworks/flux/pull/1620
[#1630]: https://github.com/weaveworks/flux/pull/1630

## 1.8.2 (2018-12-19)

This holiday season release fixes a handful of annoyances, and adds an
experimental `--watch` flag for following the progress of `fluxctl
release`.

### Fixes

- Respect proxy env entries for git operations
  [weaveworks/flux#1556][#1556]
- Only push the "sync tag" when the synced revision has changed,
  avoiding spurious notifications [weaveworks/flux#1605][#1605]
- Return any sync errors for workloads in the ListControllers API
  [weaveworks/flux#1521][#1521]

### Improvements

- The experimental flag `fluxctl release --watch` shows the rollout
  progress of workloads in the release [weaveworks/flux#1525][#1525]
- The example manifests now include resource requests, to help
  Kubernetes with scheduling [weaveworks/flux#1541][#1541]
- We have a more comprehensive [example git
  repo](https://github.com/weaveworks/flux-get-started), which is
  mentioned consistently throughout the docs
  [weaveworks/flux#1527][#1527] and [weaveworks/flux#1540][#1540].
- Many clarifications and better structure in the docs
  weaveworks/flux{[#1597], [#1595], [#1563], [#1555], [#1548],
  [#1550], [#1549], [#1547], [#1508], [#1557]}
- Registry scanning produces far less log spam, and abandons scans as
  soon as possible on being throttled [weaveworks/flux#1538][#1538]

### Thanks

Thanks to @Alien2150, @batpok, @bboreham, @brantb, @camilb,
@davidkarlsen, @dbluxo, @demikl, @dholbach, @dpgeekzero, @etos,
@hiddeco, @iandotmartin, @jakubbujny, @JeremyParker, @JimPruitt,
@johnraz, @kopachevsky, @kozejonaz, @leoblanc, @marccarre,
@marcincuber, @mgazza, @michalschott, @montyz, @ncabatoff, @nmaupu,
@Nogbit, @pdeveltere, @rampreethethiraj, @rndstr, @samisq, @scjudd,
@sfrique, @Smirl, @songsak2299, @squaremo, @stefanprodan,
@stephenmoloney, @Timer, @whereismyjetpack, @willnewby for
contributions in the period up to this release.

[#1508]: https://github.com/weaveworks/flux/pull/1508
[#1521]: https://github.com/weaveworks/flux/pull/1521
[#1525]: https://github.com/weaveworks/flux/pull/1525
[#1527]: https://github.com/weaveworks/flux/pull/1527
[#1538]: https://github.com/weaveworks/flux/pull/1538
[#1540]: https://github.com/weaveworks/flux/pull/1540
[#1541]: https://github.com/weaveworks/flux/pull/1541
[#1547]: https://github.com/weaveworks/flux/pull/1547
[#1548]: https://github.com/weaveworks/flux/pull/1548
[#1549]: https://github.com/weaveworks/flux/pull/1549
[#1550]: https://github.com/weaveworks/flux/pull/1550
[#1555]: https://github.com/weaveworks/flux/pull/1555
[#1556]: https://github.com/weaveworks/flux/pull/1556
[#1557]: https://github.com/weaveworks/flux/pull/1557
[#1563]: https://github.com/weaveworks/flux/pull/1563
[#1595]: https://github.com/weaveworks/flux/pull/1595
[#1597]: https://github.com/weaveworks/flux/pull/1597
[#1605]: https://github.com/weaveworks/flux/pull/1605

## 1.8.1 (2018-10-15)

This release completes the support for `HelmRelease` resources as used
by the Helm operator from v0.5 onwards.

**Note** This release bakes in `kubectl` v.1.11.3, while previous
releases used v1.9.0. Officially, `kubectl` is compatible with one
minor version before and one minor version after its own, i.e., now
v1.10-1.12. In practice, it may work fine for most purposes in a wider
range. If you run into difficulties relating to the `kubectl` version,
[contact us](README.md#help).

### Fixes

- Deal correctly with port numbers in images, when updating
  (Flux)HelmRelease resources
  [weaveworks/flux#1507](https://github.com/weaveworks/flux/pull/1507)
- Many corrections and updates to the documentation
  [weaveworks/flux#1506](https://github.com/weaveworks/flux/pull/1506),
  [weaveworks/flux#1502](https://github.com/weaveworks/flux/pull/1502),
  [weaveworks/flux#1501](https://github.com/weaveworks/flux/pull/1501),
  [weaveworks/flux#1498](https://github.com/weaveworks/flux/pull/1498),
  [weaveworks/flux#1492](https://github.com/weaveworks/flux/pull/1492),
  [weaveworks/flux#1490](https://github.com/weaveworks/flux/pull/1490),
  [weaveworks/flux#1488](https://github.com/weaveworks/flux/pull/1488),
  [weaveworks/flux#1489](https://github.com/weaveworks/flux/pull/1489)
- The metrics exported by the Flux daemon are now listed
  [weaveworks/flux#1483](https://github.com/weaveworks/flux/pull/1483)

### Improvements

- `HelmRelease` resources are treated as workloads, so they can be
  automated, and updated with `fluxctl release ...`
  [weaveworks/flux#1382](https://github.com/weaveworks/flux/pull/1382)
- Container-by-container releases, as used by `fluxctl --interactive`,
  now post detailed notifications to Weave Cloud
  [weaveworks/flux#1472](https://github.com/weaveworks/flux/pull/1472)
  and have better commit messages
  [weaveworks/flux#1479](https://github.com/weaveworks/flux/pull/1479)
- Errors encountered when applying manifests are reported in the
  ListControllers API (and may appear, in the future, in the `fluxctl
  release` output)
  [weaveworks/flux#1410](https://github.com/weaveworks/flux/pull/1410)

### Thanks

Thanks go to @Ashiroq, @JimPruitt, @MansM, @Morriz, @Smirl, @Timer,
@aytekk, @bzon, @camilb, @claude-leveille, @demikl, @dholbach,
@endrec, @foot, @hiddeco, @jrcole2884, @lelenanam, @marcusolsson,
@mellena1, @montyz, @olib963, @rade, @rndstr, @sfitts, @squaremo,
@stefanprodan, @whereismyjetpack for their contributions.

## 1.8.0 (2018-10-25)

This release includes a change to how image registries are scanned for
metadata, which should reduce the amount of polling, while being
sensitive to image metadata that changes frequently, as well as
respecting throttling.

### Fixes

- Better chance of a graceful shutdown on signals
  [weaveworks/flux#1438](https://github.com/weaveworks/flux/pull/1438)
- Take more notice of possible errors
  [weaveworks/flux#1432](https://github.com/weaveworks/flux/pull/1432)
  and
  [weaveworks/flux#1433](https://github.com/weaveworks/flux/pull/1433)
- Report the problematic string when failing to parse an image ref
  [weaveworks/flux#1407](https://github.com/weaveworks/flux/pull/1433)

### Improvements

- Apply CustomResourceDefinition manifests ahead of (most) other kinds
  of resource, since there will likely be other things that depend on
  the definition (e.g., the custom resources themselves)
  [weaveworks/flux#1429](https://github.com/weaveworks/flux/pull/1429)
- Add `--git-timeout` flag for setting the default timeout for git
  operations (useful e.g., if you know `git clone` will take a long
  time)
  [weaveworks/flux#1416](https://github.com/weaveworks/flux/pull/1416)
- `fluxctl list-controllers` now has an alias `fluxctl
  list-workloads` [weaveworks/flux#1425](https://github.com/weaveworks/flux/pull/1425)
- Adapt the sampling rate for image metadata, and back off when
  throttled
  [weaveworks/flux#1354](https://github.com/weaveworks/flux/pull/1354)
- The detailed rollout status of workloads is now reported in the API
  (NB this is not yet used in the command-line tool)
  [weaveworks/flux#1380](https://github.com/weaveworks/flux/pull/1380)

### Thanks

A warm thank-you to @AugustasV, @MansM, @Morriz, @MrYadro, @Timer,
@aaron-trout, @bhavin192, @brandon-bethke-neudesic, @brantb, @bzon,
@dbluxo, @dholbach, @dlespiau, @endrec, @hiddeco, @justdavid,
@justinbarrick, @kozejonaz, @lelenanam, @leoblanc, @marcemq,
@marcusolsson, @mellena1, @mt-inside, @ncabatoff, @pcfens, @rade,
@rndstr, @sc250024, @sfrique, @skurtzemann, @squaremo, @stefanprodan,
@stephenmoloney, @timthelion, @tlvu, @whereismyjetpack, @white-hat,
@wstrange for your contributions.

## 1.7.1 (2018-09-26)

This is a patch release, mainly to include the fix for initContainer
images (#1372).

### Fixes

- Include initContainers when scanning for images to fetch metadata
  for, e..g, so there will be "available image" rows for the
  initContainer in `fluxctl list-images`
  [weaveworks/flux#1372](https://github.com/weaveworks/flux/pull/1372)
- Turn memcached's logging verbosity down, in the example deployment
  YAMLs [weaveworks/flux#1369](https://github.com/weaveworks/flux/pull/1369)
- Remove mention of an archaic `fluxctl` command from help text
  [weaveworks/flux#1389](https://github.com/weaveworks/flux/pull/1389)

### Thanks

Thanks for fixes go to @alanjcastonguay, @dholbach, and @squaremo.

## 1.7.0 (2018-09-17)

This release has a soupçon of bug fixes. It gets a minor version bump,
because it introduces a new flag, `--listen-metrics`.

### Fixes

- Updates to workloads using initContainers can now succeed
  [weaveworks/flux#1351](https://github.com/weaveworks/flux/pull/1351)
- Port forwarding to GCP (and possibly others) works as intended
  [weaveworks/flux#1334](https://github.com/weaveworks/flux/issues/1334)
- No longer falls over if the directory given as `--git-path` doesn't
  exist
  [weaveworks/flux#1341](https://github.com/weaveworks/flux/pull/1341)
- `fluxctl` doesn't try to connect to the cluster when just reporting
  its version
  [weaveworks/flux#1332](https://github.com/weaveworks/flux/pull/1332)
- Metadata for unusable images (e.g., those for the wrong
  architecture) are now correctly recorded, so that they don't get
  fetched continually
  [weaveworks/flux#1304](https://github.com/weaveworks/flux/pull/1304)

### Improvements

- Prometheus metrics can be exposed on a port different from that of
  the Flux API, using the flag `--listen-metrics`
  [weaveworks/flux#1325](https://github.com/weaveworks/flux/pull/1325)

### Thanks

Thank you to the following for contributions (along with anyone I've
missed): @ariefrahmansyah, @brantb, @casibbald, @davidkarlsen,
@dholbach, @hiddeco, @justinbarrick, @kozejonaz, @lelenanam,
@petervandenabeele, @rade, @rndstr, @squaremo, @stefanprodan,
@the-fine.

## 1.6.0 (2018-08-31)

This release improves existing features, and has some new goodies like
regexp tag filtering and multiple sync paths. Have fun!

We also have a [new contributing guide](./CONTRIBUTING.md).

### Fixes

- Update example manifests to Kubernetes 1.9+ API versions
  [weaveworks/flux#1322](https://github.com/weaveworks/flux/pull/1322)
- Operate with more restricted RBAC permissions
  [weaveworks/flux#1298](https://github.com/weaveworks/flux/pull/1298)
- Verify baked-in host keys (against known good fingerprints) during
  build
  [weaveworks/flux#1283](https://github.com/weaveworks/flux/pull/1283)
- Support authentication for GKE, AWS, etc., when `fluxctl` does
  automatic port forwarding
  [weaveworks/flux#1284](https://github.com/weaveworks/flux/pull/1284)
- Respect tag filters in `fluxctl release ...`, unless `--force` is
  given
  [weaveworks/flux#1270](https://github.com/weaveworks/flux/pull/1270)

### Improvements

- Cope with `':'` characters in resource names
  [weaveworks/flux#1282](https://github.com/weaveworks/flux/pull/1282)
- Accept multiple `--git-path` arguments; sync (and update) files in
  all the paths given
  [weaveworks/flux#1297](https://github.com/weaveworks/flux/pull/1297)
- Use image pull secrets attached to service accounts, as well as
  those attached to workloads themselves
  [weaveworks/flux#1291](https://github.com/weaveworks/flux/pull/1291)
- You can now filter images using regular expressions (in addition to
  semantic version ranges, and glob patterns)
  [weaveworks/flux#1292](https://github.com/weaveworks/flux/pull/1292)

### Thanks

Thank you to the following for contributions: @Alien2150,
@ariefrahmansyah, @brandon-bethke-neudesic, @bzon, @dholbach,
@dkerwin, @hartmut-pq, @hiddeco, @justinbarrick, @petervandenabeele,
@nicolerenee, @rndstr, @squaremo, @stefanprodan, @stephenmoloney.

## 1.5.0 (2018-08-08)

This release adds semver image filters, makes it easier to use
`fluxctl` securely, and has an experimental interactive mode for
`fluxctl release`. It also fixes some long-standing problems with
image metadata DB, including no longer being bamboozled by Windows
images.

### Fixes

- Read the fallback image credentials every time, so they can be
  updated. This makes it feasible to mount them from a ConfigMap, or
  update them with a sidecar
  [weaveworks/flux#1230](https://github.com/weaveworks/flux/pull/1230)
- Take some measures to prevent spurious image updates caused by bugs
  in image metadata fetching:
  - Sort images with zero timestamps correctly
     [weaveworks/flux#1247](https://github.com/weaveworks/flux/pull/1247)
  - Skip any updates where there's suspicious-looking image metadata
    [weaveworks/flux#1249](https://github.com/weaveworks/flux/pull/1249)
    (then [weaveworks/flux#1250](https://github.com/weaveworks/flux/pull/1250))
  - Fix the bug that resulted in zero timestamps in the first place
    [weaveworks/flux#1251](https://github.com/weaveworks/flux/pull/1251)
- Respect `'false'` value for automation annotation
  [weaveworks/flux#1264](https://github.com/weaveworks/flux/pull/1264)
- Cope with images that have a Windows (or other) flavour, by omitting
  the unsupported image rather than failing entirely
  [weaveworks/flux#1265](https://github.com/weaveworks/flux/pull/1265)

### Improvements

- `fluxctl` will now transparently port-forward to the Flux pod,
  making it easier to connect securely to the Flux API
  [weaveworks/flux#1212](https://github.com/weaveworks/flux/pull/1212)
- `fluxctl release` gained an experimental flag `--interactive` that
  lets you toggle each image update on or off, then apply exactly the
  updates you have chosen
  [weaveworks/flux#1231](https://github.com/weaveworks/flux/pull/1231)
- Flux can now report and update `initContainers`, and a wider variety
  of Helm charts (as used in `FluxHelmRelease` resources)
  [weaveworks/flux#1258](https://github.com/weaveworks/flux/pull/1258)
- You can use [semver (Semantic Versioning)](https://semver.org/) filters
  for automation, rather than having to rely on glob patterns
  [weaveworks/flux#1266](https://github.com/weaveworks/flux/pull/1266)

### Thanks

Thanks to @ariefrahmansyah, @chy168, @cliveseldon, @davidkarlsen,
@dholbach, @errordeveloper, @geofflamrock, @grantbachman, @grimesjm,
@hiddeco, @jlewi, @JoeyX-u, @justinbarrick, @konfiot, @malvex,
@marccampbell, @marctc, @mt-inside, @mwhittington21, @ncabatoff,
@rade, @rndstr, @squaremo, @srikantheee84, @stefanprodan,
@stephenmoloney, @TheJaySmith (and anyone I've missed!) for their
contributions.

## 1.4.2 (2018-07-05)

This release includes a number of usability improvements, the majority
of which were suggested or contributed by community members. Thanks
everyone!

### Fixes

- Don't output fluxd usage text twice
  [weaveworks/flux#1183](https://github.com/weaveworks/flux/pull/1183)
- Allow dots in resource IDs; e.g., `default:deployment/foo.db`, which
  is closer to what Kubernetes allows
  [weaveworks/flux#1197](https://github.com/weaveworks/flux/pull/1197)
- Log more about why git mirroring fails
  [weaveworks/flux#1171](https://github.com/weaveworks/flux/pull/1171)

### Improvements

- Interpret FluxHelmRelease resources that specify multiple images to
  use in a chart
  [weaveworks/flux#1175](https://github.com/weaveworks/flux/issues/1175)
  (and several PRs that can be tracked down from there)
- Add an experimental flag for restricting the view fluxd has of the
  cluster, reducing Kubernetes API usage: `--k8s-namespace-whitelist`
  [weaveworks/flux#1184](https://github.com/weaveworks/flux/pull/1184)
- Share more image layers between quay.io/weaveworks/flux and
  quay.io/weaveworks/helm-operator images
  [weaveworks/flux#1192](https://github.com/weaveworks/flux/pull/1192)
- Apply resources in "dependency order" so that e.g., namespaces are
  created before things in the namespaces
  [weaveworks/flux#1117](https://github.com/weaveworks/flux/pull/1117)

## 1.4.1 (2018-06-21)

This release fixes some wrinkles in the new YAML updating code, so
that YAML multidocs and kubernetes List resources are fully
supported.

It also introduces the `fluxctl sync` command, which tells Flux to
update from git and apply to Kubernetes -- as requested in
[TGI Kubernetes](https://www.youtube.com/watch?v=aQz3H9bIH8Y)!

### Fixes

- Write whole files back after updates, so that multidocs and Lists
  aren't overwritten. A symptom of the problem was that a release
  would return an error something like "Verification failed: resources
  {...} were present before update and not after"
  [weaveworks/flux#1137](https://github.com/weaveworks/flux/pull/1137)
- Interpret and update CronJob manifests correctly
  [weaveworks/flux#1133](https://github.com/weaveworks/flux/pull/1133)

### Improvements

- Return a more helpful message when Flux can't parse YAML files
  [weaveworks/flux#1141](https://github.com/weaveworks/flux/pull/1141)
- Bake SSH config into the global location (`/etc/ssh`), so that it's
  easier to override it by mounting a ConfigMap into `/root/.ssh/`
  [weaveworks/flux#1154](https://github.com/weaveworks/flux/pull/1154)
- Reduce the size of list-images API/RPC responses by sending only the
  image metadata that's requested
  [weaveworks/flux#913](https://github.com/weaveworks/flux/issues/913)

## 1.4.0 (2018-06-05)

This release includes a rewrite of the YAML updating code, removing
the restrictions on using List resources and files with multiple YAML
documents, as well as fixing various bugs (like being confused by the
indentation of `container` blocks).

See https://github.com/weaveworks/flux/blob/1.4.0/site/requirements.md
for remaining constraints.

The YAML parser preserves comments and literal quoting, but may
reindent blocks the first time it changes a file.

### Fixes

- Correct an issue the led to Flux incorrectly reporting resources as
  read-only [weaveworks/flux#1119](https://github.com/weaveworks/flux/pull/1119)
- Some YAML update problems were fixed by the rewrite, the most egregious being:
  - botched releases when a YAML has indented container blocks
    [weaveworks/flux#1082](https://github.com/weaveworks/flux/issues/1082)
  - mangled annotations when using multidoc YAML files
    [weaveworks/flux#1044](https://github.com/weaveworks/flux/issues/1044)

### Improvements

- Rewrite the YAML update code to use a round-tripping parser, rather
  than regular expressions
  [weaveworks/flux#976](https://github.com/weaveworks/flux/pull/976). This
  removes the restrictions on how YAMLs are formatted, though there
  are still going to be corner cases in the parser
  ([verifying changes](https://github.com/weaveworks/flux/pull/1094)
  will mitigate those by failing updates that would corrupt files).

## 1.3.1 (2018-05-29)

### Fixes

- Correct filtering of Helm charts when loading manifests from the git repo [weaveworks/flux#1076](https://github.com/weaveworks/flux/pull/1076)
- Sync with cluster as soon as the git repository is ready [weaveworks/flux#1060](https://github.com/weaveworks/flux/pull/1060)
- Avoid panic when reporting on `StatefulSet` status [weaveworks/flux#1062](https://github.com/weaveworks/flux/pull/1062)

### Improvements

- Changes made to the git repo when releasing new images are now verified, meaning less chance of erroneous changes being committed [weaveworks/flux#1094](https://github.com/weaveworks/flux/pull/1094)
- The ListImages API method now accepts an argument saying which fields to include for each container. This is intended to cut down the amount of data sent over the wire, since you don't always need the full list of available images [weaveworks/flux#1084](https://github.com/weaveworks/flux/pull/1084)
- Add (back) the fluxd flag `--docker-config` so that image registry credentials can be supplied in a file mounted into the container [weaveworks/flux#1065](https://github.com/weaveworks/flux/pull/1065). This should make it easier to work around situations in which you don't want to use imagePullSecrets on each resource.
- Label `flux` and `helm-operator` images with [Open Containers Initiative (OCI) metadata](https://github.com/opencontainers/image-spec/blob/master/annotations.md) [weaveworks/flux#1075](https://github.com/weaveworks/flux/pull/1075)

## 1.3.0 (2018-04-26)

### Fixes

- Exclude no-longer relevant changes from auto-releases [weaveworks/flux#1036](https://github.com/weaveworks/flux/pull/1036)
- Make release and auto-release events more accurately record the
  affected resources, by looking at the calculated result [weaveworks/flux#1050](https://github.com/weaveworks/flux/pull/1050)

### Improvements

- Let the Flux daemon operate without a git repo, and report cluster resources as read-only when there is no corresponding manifest [weaveworks/flux#962](https://github.com/weaveworks/flux/pull/962)
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

- Alpha release of
  [helm-operator](https://github.com/weaveworks/flux/blob/master/site/helm/helm-integration.md). See
  [./CHANGELOG-helmop.md](./CHANGELOG-helmop.md) for future releases.

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
- When multiple Flux daemons share the same configuration repository,
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

This release introduces significant changes to the way Flux works:

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
-   Better behaviour when Flux deploys itself
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

