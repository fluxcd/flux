# fluxy

Work with your code, from laptop to prod.

## User stories

I want to update a specific image in a specific service, and deploy it.

```
fluxctl release --service=S --update-image=I
```

I want to deploy specific service/image pairs from some other source of truth, e.g. a dev environment to a prod environment.

```
# Some script wrapping this command:
fluxctl release --service=S --update-image=I
```

I want to deploy the latest images for a given service.

```
fluxctl release --service=S --update-all-images
```

I want to release a specific image to all services that are using that image, except some services that I have manually excluded somehow.

```
fluxctl release --all --update-image=I
```

I want to deploy the latest images for all services on the platform, except some services that I have manually excluded somehow.

```
fluxctl release --all --update-all-images
```

I want to deploy a service with no change of image, just taking the latest resource definition file.
This may be known as a config change deployment.

```
fluxctl release --service=S
```

I want to automatically deploy the latest images for a set of opt-in services.

```
fluxctl automate --service=S
```

I want to show all recognized services and their status.

```
fluxctl list-services
```

I want to find out what images are available for a service.

```
fluxctl list-images --service=S
```

I want to inspect the history of actions taken with Fluxy, both per-service and overall.

```
fluxctl history [--service=S]
```

## Installing

For the minute you will have to build or use the container image
`quay.io/weaveworks/fluxy`. The directory [`deploy/`](https://github.com/weaveworks/fluxy/tree/master/deploy) has example Kubernetes configuration and instructions for using it.

## Developing

### Build

Ensure the repository is checked out into $GOPATH/src/github.com/weaveworks/fluxy.
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
