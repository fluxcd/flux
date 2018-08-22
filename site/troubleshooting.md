---
title: Troubleshooting Weave Flux
menu_order: 50
---

Also see the [issues labeled with
`FAQ`](https://github.com/weaveworks/flux/labels/FAQ), which often
explain workarounds.

### `fluxctl` returns a 500 Internal Server Error

This usually indicates there's a bug in the flux daemon somewhere -- in which case please [tell us about it](https://github.com/weaveworks/flux/issues/new)!

### Flux answers everything with `git repo is not configured`

This means Flux can't read from and write to the git repo. Check that

 - ... you've supplied a git repo URL. If it's of the form
   `https://github.com/user/repo` then you will need to use the
   SSH-style URL, `git@github.com:user/repo` instead.

 - ... the deploy key has read/write access to the repo. In
   GitHub, deploy keys are installed in the settings for a
   repository. To get the deploy key Flux is using, use `fluxctl
   identity`.

 - ... that the host where your git repo lives is in
   `~/.ssh/known_hosts` in the fluxd container. We prime the container
   _image_ with host keys for `github.com`, `gitlab.com` and
   `bitbucket.org`, but if you're using your own git server, you'll
   need to add its host key. See
   [./standalone/setup.md](./standalone/setup.md#using-a-private-git-host).

### "The request failed authentication"

If you're using [Weave Cloud](https://cloud.weave.works/), this
probably means you haven't supplied the token. You can get the token
from the settings in Weave Cloud; set the environment variable
`FLUX_TOKEN` to the token.

If you have set Flux up standalone (as in the instructions in
[./standalone/installing.md](./standalone/installing.md)), this
probably means Flux is defaulting to using Weave Cloud because you've
not set the environment variable `FLUX_URL` to point at the
daemon. See [./standalone/setup.md](./standalone/setup.md).

### I'm using GCR/GKE and I keep seeing "Quota exceeded" in logs

GCP (in general) has quite conservative API rate limiting, and Flux's
default settings can bump API usage over the limits. See
[weaveworks/flux#1016](https://github.com/weaveworks/flux/issues/1016)
for advice.
