---
title: Building Weave Flux
menu_order: 80
---

# Build

Ensure the repository is checked out into $GOPATH/src/github.com/weaveworks/flux.
Then, from the root,

```
$ dep ensure
# .. time passes ..
$ make
```

This makes Docker images, and installs binaries to $GOPATH/bin.

# Test

```
$ make test
```

Note: In order to run the NATS message bus tests (the message bus that
connects fluxctl -> fluxsvc -> nats -> fluxsvc -> flux) you need to
have a running gnatsd instance.

E.g.
```
docker run -d -p 4222:4222 -p 6222:6222 --name nats-main nats
```

# Dependency management

We use [dep](https://github.com/golang/dep) to manage vendored dependencies.
Note that **we do not check in the dependencies**.

To get all the dependencies put in the `vendor/` folder, use

```
$ dep ensure
```

To add dependencies, use

```
$ dep ensure -add dependency
```
