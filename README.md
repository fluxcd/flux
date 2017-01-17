# Flux

Flux is a tool for deploying container images to Kubernetes clusters.

## Installing

For the minute you will have to build or use the container images `weaveworks/flux{d,svc}`.
The directory [`deploy/`](https://github.com/weaveworks/flux/tree/master/deploy)
 has example Kubernetes configuration and instructions for using it.

## Developing

### Build

Ensure the repository is checked out into $GOPATH/src/github.com/weaveworks/flux.
Then, from the root,

```
$ gvt restore
# .. time passes ..
$ make
```

This makes Docker images, and installs binaries to $GOPATH/bin.

### Test

```
$ go test ./...
```

### Dependency management

We use [gvt](https://github.com/FiloSottile/gvt) to manage vendored dependencies.
Note that **we do not check in the vendor folder**.

To get all the dependencies put in the `vendor/` folder, use

```
$ go get -u github.com/FiloSottile/gvt # install gvt if you don't have it
$ gvt restore
```

To add dependencies, use

```
$ gvt fetch <dependency>
```

`gvt` does not *discover* dependencies for you, but it will add them
recursively; so, it should be sufficient to just add packages you
import.

### Releasing

See the [release docs](./docs/releasing.md) for instructions about how to release a version of flux.

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
