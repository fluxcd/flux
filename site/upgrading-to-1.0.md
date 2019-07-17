---
title: Upgrading to Flux v1
menu_order: 100
---

# Upgrading to Flux v1

Flux v1 is a major improvement over the previous versions, and is
different enough that you need to do a bit of work to upgrade it.

In previous releases of Flux, much of the work was done by the
service. This meant that to get a useful system, you had to run both
the daemon and the service in your cluster, or connect the daemon to
Weave Cloud. In version 1, the daemon does all of the mechanical
work by itself, and Weave Cloud “merely” adds a web user interface and
integrations, e.g., with Slack.

## Differences between Old Flux and Flux v1

In version 1 the daemon is more self-sufficient and easier to set
up. It is also more capable, and in particular, it now synchronises
your cluster with the manifests you keep in git -- enabling you to use
git (and GitHub) workflows to manage your cluster.

<table>
 <thead>
  <tr><th>Old Flux</th><th>Flux v1</th></tr>
 </thead>
 <tr><th>Setting up the repo</th></tr>
 <tr>
  <td>
   <ul>
    <li>Create an SSH keypair</li>
    <li>Construct a YAML file with the git repo and private key in it</li>
    <li>Feed the config YAML file to the Flux service</li>
    <li>Add the public key to GitHub/wherever</li>
   </ul>
  </td>
  <td>
   <ul>
    <li>The git repo can be supplied as an argument</li>
    <li>The daemon  creates an SSH keypair</li>
    <li>Add the public key to GitHub/wherever</li>
   </ul>
  </td>
 </tr>
 <tr><th>Supplying Docker registry credentials</th></tr>
 <tr>
  <td>
   <ul>
    <li>Assemble Docker credentials in a config.json file</li>
    <li>Translate that file into entries in the config YAML file</li>
    <li>Feed the config YAML file to the Flux service (again)</li>
   </ul>
  </td>
  <td>
    The daemon finds credentials for itself by looking at Kubernetes resources
  </td>
 </tr>
 <tr><th>Managing your cluster with Flux</th></tr>
 <tr>
  <td>
   <ul>
    <li>For releasing images, use the UI or fluxctl; Flux will apply the changes to the cluster</li>
    <li>For other changes, commit them to config, then apply to the cluster with kubectl</li>
   </ul>
  </td>
  <td>
   <ul>
    <li>For releasing images, use the UI or fluxctl; Flux will commit changes to your git repo</li>
    <li>For other changes, commit them to your git repo</li>
    <li>Flux applies all changes to the git repo to the cluster</li>
   </ul>
  </td>
 </tr>
</table>

## Upgrade process

In summary, you will need to:

 1. Remove the old Flux resources from your cluster
 2. Delete any deployment keys
 3. Run the new Flux resources
 4. Install a new deploy key

First, it will help in a few places to have an old fluxctl
around. Download it from GitHub:

```sh
curl -o fluxctl_030 https://github.com/weaveworks/flux/releases/download/0.3.0/fluxctl_linux_amd64
# or if using macOS,
# curl -o fluxctl_030 https://github.com/weaveworks/flux/releases/download/0.3.0/fluxctl_darwin_amd64
chmod a+x ./fluxctl_030
```

The next steps depend on whether you

 * [Connect Flux to Weave Cloud](#if-you-are-connecting-to-weave-cloud); or,
 * [You are running Flux standalone](#if-you-are-running-flux-in-standalone-mode) (i.e., you run both
   the Flux daemon and the Flux service, rather than going through
   Weave Cloud).

## <a name="weavecloud">If you are connecting to Weave Cloud</a>

Set the environment variable `FLUX_SERVICE_TOKEN` to be your service
token (found in the Weave Cloud UI for the instance you are
upgrading).

Before making changes, get the config so that it can be consulted
later:

```sh
./fluxctl_030 get-config --fingerprint=md5 | tee old-config.yaml
```

### Remove old Flux resources

> *Important! If you have Flux resources committed to git* The first
> thing to do is remove any manifests for running Flux that you have
> stored in git, before deleting them in the cluster
> (below). Otherwise, when the new Flux daemon runs it will restore
> the old configuration.

Run Weave Cloud’s launch generator to delete the resources in the
cluster:

```sh
kubectl delete -n kube-system -f \
  https://cloud.weave.works/k8s/flux.yaml?flux-version=0.3.0”
```

### Delete deployment keys

It’s good practice to remove any unused deployment keys. If you’re
using GitHub, look at the settings for the repository you were
pointing Flux at, and delete the key Flux was using.

To check you are removing the correct key, get the fingerprint of the
key used by Flux with

```sh
./fluxctl_030 get-config --fingerprint=md5
```

### Configure and run the new Flux resources

> First, it is important to understand that Flux manages more of your
> cluster resources now. It will automatically apply manifests that
> appear in your config repo, either by creating or by updating them. In
> other words, it tries to keep the cluster running whatever is
> represented in the repo. (Though it doesn’t delete things, yet.)

To run the new Flux with Weave Cloud:

 * Go to your instance settings (the cog icon) and click the “Config”
   then “Deploy” menu items
 * Enter the git URL, path and branch values from the saved config
   (in `old-config.yaml`)
 * Run the `kubectl` command shown below the form.
 * Following the instructions underneath the command, to install the
   deploy key (this is also a good opportunity to delete any old keys,
   if you didn’t do that above).
 * Wait for the big red "Agent not configured" message to clear.

You should now be able to click the “Deploy” tab at the top and see
your running system (again), with the updated Flux daemon.

## If you are running Flux in "standalone" mode

Set the environment variable FLUX_URL to point to the Flux service you
are running, as described in
[the old deployment docs](https://github.com/weaveworks/flux/blob/0.3.0/site/using.md#fluxctl-setup). The
particular URL will differ, depending on how you have told Kubernetes
to expose the Flux service.

Before making any changes, get the config so that it can be consulted later:

```
./fluxctl_030 get-config --fingerprint=md5 | tee old-config.yaml
```

### Remove old Flux resources

> *Important! If you have Flux resources committed to git*
>
> The first thing to do here is to remove any manifests for running
> Flux you have stored in git, before deleting them in the cluster
> (below). If you don’t remove these, running the new Flux daemon will
> restore the old configuration.

You can delete the Flux resources by referring to the manifest files
used to create them. If you don’t have the files on hand, you can try
using the example deployment as a stand-in:

```sh
git clone --branch 0.3.0 git@github.com:weaveworks/flux flux-0.3.0
kubectl delete --ignore-not-found -R -f ./flux-0.3.0/deploy
```

That’s something of a sledgehammer! But it should cover most cases.

### Delete deployment keys

It’s good practice to remove any unused deployment keys. If you’re
using GitHub, look at the settings for the repository you were
pointing Flux at, and delete the key Flux was using. To check you are
removing the correct key, you can see the fingerprint of the key used
by Flux in the file `old-config.yaml` that was created earlier.

### Configure and run new Flux resources

> First, it is important to understand that Flux manages more of your
> cluster resources now. It will automatically apply the manifests
> that appear in your config repo, either by creating or by updating
> them.  In other words, it tries to keep the cluster running whatever
> is represented in the repo. (Though it doesn’t delete things, yet).

To run Flux without connecting to Weave Cloud, adapt the manifests
provided in the
[Flux repo](https://github.com/weaveworks/flux/tree/master/deploy)
with the git parameters (URL, path, and branch) from
`old-config.yaml`, and then apply them with `kubectl`. Consider adding
these adapted manifests to your own config repo.

The daemon now has an API itself, so you can point fluxctl directly at
it (the example manifests include a Kubernetes service so you can do
just that).

You may find that you need to set FLUX_URL again, to take account of
the new deployment. See the
[setup instructions](https://github.com/weaveworks/flux/blob/1.0.1/site/standalone/setup.md#connecting-fluxctl-to-the-daemon)
for guidance.

To see the SSH key created by Flux, download the latest fluxctl from
[the release page](https://github.com/weaveworks/flux/releases/tag/1.0.1)
and run:

```sh
fluxctl identity
```

You will need to add this as a deploy key, which is also described in
the setup instructions linked above.

## Troubleshooting

### The `kubectl delete` commands didn’t delete anything

It’s possible that the Flux resources are in an unusual namespace or
given a different name. As a last resort, you can hunt down the
resources by name and delete them. Weave Cloud’s “Explore” tab may
help; or use kubectl to look for likely suspects.

```sh
kubectl get serviceaccount,service,deployment --all-namespaces
```

Have a look for deployments and services with “flux” in the name.

### I deleted the Flux resources but when I install Flux v1 they come back

The most likely explanation is that you have manifests for the
resources in your config repo. When Flux v1 starts, it does a sync --
and if there are manifests for the old Flux still in git, it will
create those as resources.

If that’s the case, you will need to remove the manifests from git
before running Flux v1.
