---
title: Building Weave Flux
menu_order: 80
---

# Build

You'll need a working `go` environment (official releases are build against `1.10`),
including the [`dep`](https://github.com/golang/dep#installation) tool.

It's also expected that you have a Docker daemon for building images.

Ensure the repository is checked out into $GOPATH/src/github.com/weaveworks/flux.
Then, from the root,

```sh
$ dep ensure
# .. time passes ..
$ make
```

This makes Docker images, and installs binaries to $GOPATH/bin.

# Test

```sh
$ make test
```

# Dependency management

We use [dep](https://github.com/golang/dep) to manage vendored dependencies.
Note that **we do not check in the dependencies**.

To get all the dependencies put in the `vendor/` folder, use

```sh
$ dep ensure
```

If you see a big diff or other unexpected output after running `dep ensure`,
make sure you're using the latest official release of `dep`.
