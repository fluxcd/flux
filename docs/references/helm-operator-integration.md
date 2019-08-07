# Integration with the Helm operator

You can release charts to your cluster via "GitOps", by combining Flux
and the Helm operator (also in [fluxcd/flux](https://github.com/fluxcd/flux)).

The essential mechanism is this: the declaration of a Helm release is
represented by a custom resource, specifying the chart and its
values. If you put such a resource in your git repo as a file, Flux
will apply it to the cluster, and once it's in the cluster, the Helm
Operator will make sure the release exists by installing or upgrading
it.

## Upgrading images in a `HelmRelease` using Flux

If the chart you're using in a `HelmRelease` lets you specify the
particular images to run, you will usually be able to update them with
Flux, the same way you can with Deployments and so on.

> **Note:** for automation to work, the repository _and_ tag should be
> defined, as Flux determines image updates based on what it reads in
> the `.spec.values` of the `HelmRelease`.

Flux interprets certain commonly used structures in the `values`
section of a `HelmRelease` as referring to images. The following
are understood (showing just the `values` section):

```yaml
values:
  image: repo/image:version
```

```yaml
values:
  image: repo/image
  tag: version
```

```yaml
values:
  registry: docker.io
  image: repo/image
  tag: version
```

```yaml
values:
  image:
    repository: repo/image
    tag: version
```

```yaml
values:
  image:
    registry: docker.io
    repository: repo/image
    tag: version
```

These can appear at the top level (immediately under `values:`), or in
a subsection (under a key, itself under `values:`). Other values
may be mixed in arbitrarily. Here's an example of a values section
that specifies two images, along with some other configuration:

```yaml
values:
  persistent: true

  # image that will be labeled "chart-image"
  image: repo/image1:version

  subsystem:
    # image that will be labeled "subsystem"
    image:
      repository: repo/image2
      tag: version
      imagePullPolicy: IfNotPresent
    port: 4040
```

You can use the [same annotations](fluxctl.md) in
the `HelmRelease` as you would for a Deployment or other workload,
to control updates and automation. For the purpose of specifying
filters, the container name is either `chart-image` (if at the top
level), or the key under which the image is given (e.g., `"subsystem"`
from the example above).

Top level image example:

```yaml
kind: HelmRelease
metadata:
  annotations:
    flux.weave.works/automated: "true"
    flux.weave.works/tag.chart-image: semver:~4.0
spec:
  values:
    image:
      repository: bitnami/mongodb
      tag: 4.0.3
```

Sub-section images example:

```yaml
kind: HelmRelease
metadata:
  annotations:
    flux.weave.works/automated: "true"
    flux.weave.works/tag.prometheus: semver:~2.3
    flux.weave.works/tag.alertmanager: glob:v0.15.*
    flux.weave.works/tag.nats: regex:^0.6.*
spec:
  values:
    prometheus:
      image: prom/prometheus:v2.3.1
    alertmanager:
      image: prom/alertmanager:v0.15.0
    nats:
      image:
        repository: nats-streaming
        tag: 0.6.0
```
