---
title: Automations, locks and annotations
weight: 40
---

In this tutorial we want to get a better feel for what we can do with
Flux. We won't spend too much time with getting it up and running, so let's
get that out of the way first.

In our example we are going to use the `flux-get-started` example deployment.
So as your first step, please head to [our example
deployment](https://github.com/fluxcd/flux-get-started) and click on the
"Fork" button.

## Setup

Install [fluxctl](../references/fluxctl.md) and run (replace `YOURUSER` with your GitHub username):

```sh
export GHUSER="YOURUSER"
fluxctl install \
--git-user=${GHUSER} \
--git-email=${GHUSER}@users.noreply.github.com \
--git-url=git@github.com:${GHUSER}/flux-get-started \
--git-path=namespaces,workloads \
--namespace=flux | kubectl apply -f -
```

### Connecting to your git config

The first step is done. Flux is now and up running (you can confirm by
running `kubectl get pods --all-namespaces`).

In the second step we will use `fluxctl` to talk to Flux in the cluster and
interact with the deployments. (It enables you to drive all of Flux, so have a look at the output of
`fluxctl -h` to get a better idea.)

{{% alert %}}
Another option (without installing `fluxctl` is to take a look
at the resulting annotation changes and make the changes in Git. This is
GitOps after all. :-)
{{% /alert %}}

Tell fluxctl in which namespace is Flux installed:

```sh
export FLUX_FORWARD_NAMESPACE=flux
```

To enable Flux to sync your config, you need to add the deployment key
to your fork.

Get your Flux deployment key by running:

```sh
fluxctl identity
```

Copy/paste the key and add it to
`https://github.com/YOUR-USER-ID/flux-get-started/settings/keys/new` and
enable write access for it.

Wait for sync to happen or run:

```sh
fluxctl sync
```

## Driving Flux

After syncing, Flux will find out which workloads there are, which
images are available and what needs doing. To find out which workloads are
managed by Flux, run:

```sh
fluxctl list-workloads -a 
```

Notice that `podinfo` is on `3.1.0` and in state `automated`.

To check which images are available for podinfo run:

```sh
fluxctl list-images -w demo:deployment/podinfo
```

Now let's change the policy for `podinfo` to target `3.2` releases:

```sh
fluxctl policy -w demo:deployment/podinfo --tag='podinfod=3.2.*'
```

On the command-line you should see a message just like this one:

```sh
WORKLOAD                 STATUS   UPDATES
demo:deployment/podinfo  success
Commit pushed:  4755a3b
```

If you now go back to `https://github.com/YOUR-USER-ID/flux-get-started` in
your browser, you will notice that Flux has made a commit on your
behalf. The policy change is now in Git, which is great for transparency and
for defining expected state.

It should look a little something like this:

```diff
--- a/workloads/podinfo-dep.yaml
+++ b/workloads/podinfo-dep.yaml
@@ -8,8 +8,8 @@ metadata:
     app: podinfo
   annotations:
     fluxcd.io/automated: "true"
-    fluxcd.io/tag.podinfod: semver:~3.1
+    fluxcd.io/tag.podinfod: glob:3.2.*
```

If you have a closer look at the last change which was committed, you'll see
that the image filtering pattern has been changed. (Our docs explain how to
use `semver`, `glob`, `regex` filtering.)

Again, wait for the sync to happen or run

```sh
fluxctl sync
```

To check which image is current, run:

```sh
fluxctl list-images -w demo:deployment/podinfo
```

In our case this is `3.2.2` (it could be a later image too). Let's say an
engineer found that `3.2.2` was faulty and we have to go back to `3.2.1`.
That's easy.

Lock deployment with a message describing why:

```sh
fluxctl lock -w demo:deployment/podinfo -m "3.2.2 does not work for us"
```

The resulting diff should look like this:

```diff
--- a/workloads/podinfo-dep.yaml
+++ b/workloads/podinfo-dep.yaml
@@ -10,6 +10,7 @@ metadata:
     app: podinfo
   annotations:
     fluxcd.io/automated: "true"
     fluxcd.io/tag.podinfod: glob:3.2.*
+    fluxcd.io/locked: 'true'
 spec:
   strategy:
     rollingUpdate:
```

Rollback to `3.2.1`. Flag `--force` is needed because the workload is locked:

```sh
fluxctl release --force --workload demo:deployment/podinfo -i stefanprodan/podinfo:3.2.1
```

The response should be:

```sh
Submitting release ...
CONTROLLER               STATUS   UPDATES
demo:deployment/podinfo  success  podinfod: stefanprodan/podinfo:3.2.2 -> 3.2.1
Commit pushed:  426d723
Commit applied: 426d723
```

and the diff for this is going to look like this:

```diff
--- a/workloads/podinfo-dep.yaml
+++ b/workloads/podinfo-dep.yaml
@@ -33,7 +33,7 @@ spec:
         - "1"
       containers:
       - name: podinfod
-        image: stefanprodan/podinfo:3.2.2
+        image: stefanprodan/podinfo:3.2.1
```

And that's it. At the end of this tutorial, you have automated, locked and
annotated deployments with Flux.

Another tip, if you should get stuck anywhere: check what Flux is doing. You
can do that by simply running:

```sh
kubectl logs -n flux deploy/flux -f
```

If you should have any questions, find us on Slack in the [#flux
channel](https://cncf.slack.com/messages/flux/), get
an invite to it [here](https://slack.cncf.io).
