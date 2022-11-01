# Building Flux

> **🛑 Upgrade Advisory**
>
> This documentation is for Flux (v1) which has [reached its end-of-life in November 2022](https://fluxcd.io/blog/2022/10/september-2022-update/#flux-legacy-v1-retirement-plan).
>
> We strongly recommend you familiarise yourself with the newest Flux and [migrate as soon as possible](https://fluxcd.io/flux/migration/).
>
> For documentation regarding the latest Flux, please refer to [this section](https://fluxcd.io/flux/).

You'll need a working `go` environment version >= 1.11 (official releases are built against `1.13`).
It's also expected that you have a Docker daemon for building images.

Clone the repository. The project uses [Go Modules](https://github.com/golang/go/wiki/Modules),
so if you explicitly define `$GOPATH` you should clone somewhere else.

Then, from the root directory:

```sh
make
```

This makes Docker images, and installs binaries to `$GOBIN` (if you define it) or `$(go env GOPATH)/bin`.

> ⚠ Note:
> The default target architecture is amd64. If you would like
> to try to build Docker images and binaries for a different
> architecture you will have to set ARCH variable:
>
> ```sh
> make ARCH=<target_arch>
> ```

## Running tests

```sh
# Unit tests
make test

# End-to-end tests
make e2e
```
