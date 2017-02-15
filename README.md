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

Note: In order to run the NATS message bus tests (the message bus that connects fluxctl -> fluxsvc -> nats -> fluxsvc -> fluxd) you need to have a running gnatsd instance.

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

## <a name="help"></a>Getting Help

If you have any questions about Flux and continuous delivery:

- Invite yourself to the <a href="https://weaveworks.github.io/community-slack/" target="_blank"> #weave-community </a> slack channel.
- Ask a question on the <a href="https://weave-community.slack.com/messages/general/"> #weave-community</a> slack channel.
- Join the <a href="https://www.meetup.com/pro/Weave/"> Weave User Group </a> and get invited to online talks, hands-on training and meetups in your area.
- Send an email to <a href="mailto:weave-users@weave.works">weave-users@weave.works</a>
- <a href="https://github.com/weaveworks/flux/issues/new">File an issue.</a>

Your feedback is always welcome!
