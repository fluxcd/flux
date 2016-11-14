# Deploying Flux to Kubernetes

There's two ways to deploy Flux: you can use the hosted service at
Weave Cloud, or you can run the service yourself.

Using Weave Cloud is described in [cloud/](./cloud/README.md); running
it all yourself is described in [standalone/](README.md).

Either way, after you have got it running, you'll need to supply some
information so Flux can update your Kubernetes manifests, push
notifications through Slack, and so on.

## Creating a deploy key

Flux updates a Git repository containing your Kubernetes config each
time a service is released; this will usually require an SSH access
key.

Here is an example of setting this up for the `helloworld` example in
the Flux git repository, in the directory `testdata`. You can skip
forking the example repository if you already have one you're using.

Fork the Flux repository on github (you may also wish to rename it,
e.g., to `flux-testdata`). Now, we're going to add a deploy key so
Flux can push to that repo. Generate a key in the console:

```
ssh-keygen -t rsa -b 4096 -f id-rsa-flux
```

This makes a private key file (`id-rsa-flux`) which we'll supply to
Flux in a minute, and a public key file (`id-rsa-flux.pub`) which
we'll now give to Github.

On the Github page for your forked repo, go to the settings and find
the "Deploy keys" page. Add one, check the write access box, and paste
in the contents of the `id-rsa-flux.pub` file -- the public key.

## Uploading a configuration

To begin using Flux, you need to provide at least the git repository
and the key from earlier. The following assumes you have set the
environment variable `FLUX_SERVICE_TOKEN` if you are using Weave
Cloud, or `FLUX_URL` if you are using a standalone deployment of Flux;
add the `--token` or `--url` argument in, if you prefer to supply
those explicitly.

Get a blank config with

```sh
fluxctl get-config > flux.conf
```

Now edit the file `flux.conf` -- it'll look like this:

```yaml
git:
  URL: ""
  path: ""
  branch: ""
  key: ""
slack:
  hookURL: ""
  username: ""
registry:
  auths: {}
```

Here's an example with values filled in:

```yaml
git:
  URL: git@github.com:squaremo/flux-testdata
  path: testdata
  branch: master
  key: |
         -----BEGIN RSA PRIVATE KEY-----
         ZNsnTooXXGagxg5a3vqsGPgoHH1KvqE5my+v7uYhRxbHi5uaTNEWnD46ci06PyBz
         zSS6I+zgkdsQk7Pj2DNNzBS6n08gl8OJX073JgKPqlfqDSxmZ37XWdGMlkeIuS21
         nwli0jsXVMKO7LYl+b5a0N5ia9cqUDEut1eeKN+hwDbZeYdT/oGBsNFgBRTvgQhK
         ... contents of id-rsa-flux file from above ...
         -----END RSA PRIVATE KEY-----
slack:
  hookURL: ""
  username: ""
registry:
  auths: {}
```

Note the use of `|` to have a multiline string value for the key; all
the lines must be indented if you use that.

If you use any private Docker image repositories, you will also need
to supply authentication information for those. These are given per
registry, like the following snippet (you can nick these from the
analagous section of ~/.docker/config.json, they are just
base64-encoded `<username>:<password>`):

```yaml
# ...
registry:
  auths:
    'https://index.docker.io/v1/':
      auth: "dXNlcm5hbWU6cGFzc3dvcmQK"
```

(NB the key is a URL, and will usually have to be quoted as it is above.)

Finally, give the config to Flux:

```sh
fluxctl set-config --file=flux.conf
```
