# Driving Weave Flux - automations, locks and annotations

In this tutorial we want to get a better feel for what we can do with Weave
Flux. We won't spend too much time with getting it up and running, so let's
get that out of the way first.

In our example we are going to use the `flux-get-started` example deployment.
So as your first step, please head to [our example
deployment](https://github.com/weaveworks/flux-get-started) and click on the
"Fork" button.

## Setup

Get the source code of Weave Flux:

```sh
git clone https://github.com/weaveworks/flux
cd flux
```

In the next step, let's change the Git URL of Flux to point to our fork:

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

### Alternative: Using Helm for the setup

If you have never used Helm, you first need to

- Download/install Helm
- Set up Tiller. First create a service account and a cluster role binding
  for Tiller:

  ```sh
  kubectl -n kube-system create sa tiller
  kubectl create clusterrolebinding tiller-cluster-rule \
    --clusterrole=cluster-admin \
    --serviceaccount=kube-system:tiller
  ```

  Deploy Tiller in the `kube-system` namespace:

  ```sh
  helm init --skip-refresh --upgrade --service-account tiller
  ```

Now you can take care of the actual installation. First add the Flux
repository of Weaveworks:

```sh
helm repo add weaveworks https://weaveworks.github.io/flux
```

Apply the Helm Release CRD:

```sh
kubectl apply -f https://raw.githubusercontent.com/weaveworks/flux/master/deploy-helm/flux-helm-release-crd.yaml
```

Install Weave Flux and its Helm Operator by specifying your fork URL. Just
make sure you replace `YOURUSER` with your GitHub username in the command
below:

```sh
helm upgrade -i flux \
--set helmOperator.create=true \
--set helmOperator.createCRD=false \
--set git.url=git@github.com:YOURUSER/flux-get-started \
--namespace default \
weaveworks/flux
```

> **Note:** In this tutorial we keep things simple, so we deploy Flux into
the `default` namespace. Normally you would pick a separate namespace for
it. `fluxctl` has the `k8s-fwd-ns` option for specifying the right
namespace.

### Connecting to your git config

The first step is done. Flux is now and up running (you can confirm by
running `kubectl get pods --all-namespaces`).

In the second step we will use fluxctl to talk to Flux in the cluster and
interact with the deployments. First, please [install fluxctl](https://github.com/weaveworks/flux/blob/master/site/fluxctl.md#installing-fluxctl).
(It enables you to drive all of Weave Flux, so have a look at the output of
`fluxctl -h` to get a better idea.)

> **Note:** Another option (without installing `fluxctl` is to take a look
at the resulting annotation changes and make the changes in Git. This is
GitOps after all. :-)

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

On the command-line you should see a message just like this one:

```sh
CONTROLLER               STATUS   UPDATES
demo:deployment/podinfo  success
Commit pushed:  4755a3b
```

If you now go back to `https://github.com/YOUR-USER-ID/flux-get-started` in
your browser, you will notice that Weave Flux has made a commit on your
behalf. The policy change is now in Git, which is great for transparency and
for defining expected state.

It should look a little something like this:

```diff
--- a/workloads/podinfo-dep.yaml
+++ b/workloads/podinfo-dep.yaml
@@ -8,8 +8,8 @@ metadata:
     app: podinfo
   annotations:
     flux.weave.works/automated: "true"
-    flux.weave.works/tag.init: regexp:^3.*
-    flux.weave.works/tag.podinfod: semver:~1.3
+    flux.weave.works/tag.init: glob:1.4.*
+    flux.weave.works/tag.podinfod: glob:1.4.*
 spec:
   strategy:
     rollingUpdate:
```

If you have a closer look at the last change which was committed, you'll see
that the image filtering pattern has been changed. (Our docs explain how to
use `semver`, `glob`, `regex` filtering.)

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

The response should be

```sh
Submitting release ...
CONTROLLER               STATUS   UPDATES
demo:deployment/podinfo  success  podinfod: stefanprodan/podinfo:1.4.2 -> 1.4.1
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
-        image: stefanprodan/podinfo:1.3.2
+        image: stefanprodan/podinfo:1.4.1
         imagePullPolicy: IfNotPresent
         ports:
         - containerPort: 9898
```

Lock to `1.4.1` with a message describing why:

```sh
fluxctl lock -c demo:deployment/podinfo -m "1.4.2 does not work for us"
```

The resulting diff should look like this

```diff
--- a/workloads/podinfo-dep.yaml
+++ b/workloads/podinfo-dep.yaml
@@ -10,6 +10,7 @@ metadata:
     app: podinfo
   annotations:
     flux.weave.works/automated: "true"
     flux.weave.works/tag.init: glob:1.4.*
     flux.weave.works/tag.podinfod: glob:1.4.*
+    flux.weave.works/locked: 'true'
 spec:
   strategy:
     rollingUpdate:
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