# Automated deployment of new container images

Flux can be used to automate container image updates in your cluster.
Flux periodically scans the pods running in your cluster and builds a list of all container images.
Using the image pull secrets, it connects to the container registries, pulls the images metadata
and stores the image tag list in memcached.

You can enable the automate image tag updates by annotating your deployments, statefulsets,
daemonsets or cronjobs objects. You can also control what tags should be considered for an
update by using glob, regex or semantic version expressions.

> **Note:** that Flux only works with immutable image tags (`:latest` is not supported).
Every image tag must be unique, for this you can use the Git commit SHA or semver when tagging images.

## Examples

What follows is a list of examples on how you can control the image update automation. If you're using Helm releases 
please see the [Helm operator integration docs](helm-operator-integration.md).

Turn on automation based on timestamp:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    fluxcd.io/automated: "true"
spec:
  template:
    spec:
      containers:
      - name: app
        image: docker.io/org/my-app:1.0.0
```

The above configuration will make Flux update the `app` container when you push
a new image tag, be it `my-app:1.0.1` or `my-app:9e3bdaf`.

Restrict image updates with sem ver:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    fluxcd.io/automated: "true"
    fluxcd.io/tag.app: semver:~1.0
spec:
  template:
    spec:
      containers:
      - name: app
        image: docker.io/org/my-app:1.0.0
```

The above configuration will make Flux update the image when you push
an image tag that matches the [semantic version](https://semver.org/)
expression e.g `my-app:1.0.1` but not `my-app:1.2.0`. 

Flux also support all the other ranges and operators available [here](https://github.com/Masterminds/semver) in addition to the `~` range.

Restrict image to deploy prerelease version up until `myapp:1.0.0` but not `myapp:1.0.1`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    fluxcd.io/automated: "true"
    fluxcd.io/tag.app: "semver: >= 1.0.0-rc.0, <1.0.1"
spec:
  template:
    spec:
      containers:
      - name: app
        image: docker.io/org/my-app:1.0.0-rc.1
```

Restrict image updates with glob and regex expressions:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    fluxcd.io/automated: "true"
    fluxcd.io/tag.sidecar: regex:^stg.*
    fluxcd.io/tag.app: glob:dev-*
spec:
  template:
    spec:
      containers:
      - name: sidecar
        image: docker.io/org/my-proxy:stg-4s7bsgv
      - name: app
        image: docker.io/org/my-app:dev-9e3bdaf
```

The above configuration will make Flux update the `sidecar` when you push
a tag for the `my-proxy` image that begins with `stg`.
For the `app` container, Flux will update it when you push a tag for the
`my-app` image that begins with `dev-`.

To target a specific container the annotation format is `fluxcd.io/tag.<CONTAINER>: <TYPE>:<EXPRESSION>`.

You can turn off the automation with `fluxcd.io/automated: "false"` or with `fluxcd.io/locked: "true"`.

