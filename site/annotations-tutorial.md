# Driving Weave Flux - automations, locks and annotations

In this tutorial we want to get a better feel for what we can do with Weave
Flux. We won't spend too much time with getting it up and running, so let's
get that out of the way first.

## Setup

Get the source code of Weave Flux:

```sh
git clone https://github.com/weaveworks/flux
cd flux
```

To get a deployment up and running, it's easiest if you head to [our example
deployment](https://github.com/weaveworks/flux-get-started) we set up. Please
head to its site on Github and click on the "Fork" button.

In  the next step, let's change the Git URL of Flux to point to our fork:

```sh
EDITOR deploy/flux-deployment.yaml
```

And update the following line

```yaml
    --git-url=git@github.com:weaveworks/flux-get-started
```

to point to your fork, e.g. if your Github Login is `baloothebear`, the line
above should be

```yaml
    --git-url=git@github.com:baloothebear/flux-get-started
```

Save the file. For our simple case, that's all the configuration we need. Now
it's time to deploy Flux. Simply run

```sh
kubectl apply -f deploy
```

The first step is done. Flux is now and up running (you can confirm by
running `kubectl get pods --all-namespaces`).

In the second step we will use fluxctl to talk to Flux in the cluster and
interact with the deployments. First, please [install fluxctl](https://github.com/weaveworks/flux/blob/master/site/fluxctl.md#installing-fluxctl).
(It enables you to drive all of Weave Flux, so have a look at the output of
`fluxctl -h` to get a better idea.)

To enable Weave Flux to sync your config, you need to add the deployment key
to your fork.

Get your Flux deployment key by running

```sh
fluxctl identity
```

Copy/paste the key and add it to
`https://github.com/YOUR-USER-ID/flux-get-started/settings/keys/new` and
enable write access for it.

Wait for sync to happen or run

```sh
fluxctl sync
```

## Driving Weave Flux

After syncing, Weave Flux will find out which workloads there are, which
images are available and what needs doing. To find out which workloads are
managed by Weave Flux, run

```sh
fluxctl list-workloads -a
```

Notice that `podinfo` is on `v1.3.2` and in state `automated`.

To check which images are avaible for podinfo run

```sh
fluxctl list-images -c demo:deployment/podinfo
```

Now let's change the policy for `podinfo` to target `1.4.*` releases:

```sh
fluxctl policy -c demo:deployment/podinfo --tag-all='1.4.*'
```

If you now go back to `https://github.com/YOUR-USER-ID/flux-get-started` in
your browser, you will notice that Weave Flux has made a commit on your
behalf. The policy change is now in Git, which is great for transparency and
for defining expected state.

If you have a closer look at the last change which was committed, you'll see
that the image filtering pattern has been changed. (Our docs explain how to
use semver, glob, regex filtering.)

Again, wait for the sync to happen or run

```sh
fluxctl sync
```

To check which image is current, run

```sh
fluxctl list-images -c demo:deployment/podinfo
```

In our case this is `1.4.2` (it could be a later image too). Let's say an
engineer found that `1.4.2` was faulty and we have to go back to `1.4.1`.
That's easy.

Rollback to `1.4.1`:

```sh
fluxctl release -c demo:deployment/podinfo -i stefanprodan/podinfo:1.4.1
```

Lock to `1.4.1` with a message describing why:

```sh
fluxctl lock -c demo:deployment/podinfo -m "1.4.2 does not work for us"
```

And that's it. At the end of this tutorial, you have automated, locked and
annotated deployments with Weave Flux.

Another tip, if you should get stuck anywhere: check what Flux is doing. You
can do that by simply running

```sh
kubectl logs -n default deploy/flux -f
```

If you should have any questions, find us on Slack in the [#flux
channel](https://weave-community.slack.com/messages/flux/), get
an invite to it [here](https://slack.weave.works/).