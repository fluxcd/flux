# fluxy

Flux, reimagined.

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

