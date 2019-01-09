This is the changelog for the Flux daemon; the changelog for the Helm
operator is in [./CHANGELOG-helmop.md](./CHANGELOG-helmop.md).

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
  the flux API, using the flag `--listen-metrics`
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

