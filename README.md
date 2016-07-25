# fluxy

Flux, reimagined.

## Workflows

All workflows start with the assumption that I've got a helloworld service running on my platform,
 and I've pushed a new version of the helloworld image to my image repository,
 which I now want to release.

### Explicit

```
$ # Which services are running on my platform?
$ fluxctl service list

$ # Which images are available for the helloworld service?
$ fluxctl service images --service=helloworld

$ # Update my resource file to use a new image.
$ fluxctl config update --file helloworld-rc.yaml --image=quay.io/weaveworks/helloworld:de9f3b2 --output=helloworld-rc.yaml

$ # Release a new version of the helloworld service.
$ fluxctl service release --service=helloworld --file=helloworld-rc.yaml
```

### Interactive

Note: not yet implemented.

```
$ # I'd like to release the latest version of helloworld.
$ fluxctl service release --service=helloworld --interactive
Querying platform for "helloworld" service...

 SERVICE     IMAGE                                  STATUS
 helloworld  quay.io/weaveworks/helloworld:a000001  At rest

helloworld is at rest, OK to continue.
Looking for a resource file matching "quay.io/weaveworks/helloworld"... found 2.

     FILE                         IMAGE
  1) k8s/dev/helloworld-rc.yaml   quay.io/weaveworks/helloworld:b11111f
  2) k8s/prod/helloworld-rc.yaml  quay.io/weaveworks/helloworld:a000001

Which file(s) to update? 1
OK, will update k8s/dev/helloworld-rc.yaml.
Querying registry for "quay.io/weaveworks/helloworld" images...

    IMAGE                                  CREATED AT               RUNNING
 1) quay.io/weaveworks/helloworld:a000001  2016-07-10 10:09:53 UTC  •
 2) quay.io/weaveworks/helloworld:de9f3b2  2016-07-11 16:15:01 UTC

Which image to release? 2
OK, releasing image "quay.io/weaveworks/helloworld:de9f3b2" to service "helloworld".
Updating file 1 of 1: k8s/dev/helloworld-rc.yaml...

  -     image: quay.io/weaveworks/helloworld:b11111f
  +     image: quay.io/weaveworks/helloworld:de9f3b2

Does this look right? yes
OK, writing 1 file(s) to disk.
Perform the release? yes
Releasing with an update-period of 5s......... complete.
Be sure to commit and push your changes!
```

### Automated

```
$ # TBD
```

## Installing

If you have a working Go toolchain, you can install the most recent version of the binaries via

```
$ go get github.com/weaveworks/fluxy/cmd/fluxd
$ go get github.com/weaveworks/fluxy/cmd/fluxctl
```

Otherwise, see [the releases page](https://github.com/weaveworks/fluxy/releases) for downloads.

## Developing

### Build

Ensure the repository is checked out into $GOPATH/src/github.com/weaveworks/fluxy.
Then, from the root,

```
$ go install ./...
```

Flux vendors all of its dependencies, so that should be sufficient.
Binaries are installed to $GOPATH/bin.

### Test

```
$ go test ./...
```

### Dependency management

We use [Glide](https://github.com/Masterminds/glide) to manage vendored dependencies.
Note that **we do not check in the vendor folder**.
If you add or remove dependencies, use the following command to update the glide.yaml and glide.lock files,
 and to populate your local vendor folder.

```
$ glide update
```

### Contribution

Flux follows a typical PR workflow.
All contributions should be made as PRs that satisfy the guidelines below.

### Guidelines

- All code must abide [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Names should abide [What's in a name](https://talks.golang.org/2014/names.slide#1)
- Code must build on both Linux and Darwin, via plain `go build`
- Code should have appropriate test coverage, invoked via plain `go test`

In addition, several mechanical checks are enforced.
See [the lint script](/lint) for details.

