# Manifest factorization through `.flux.yaml` configuration files

## Enabling search of `.flux.yaml` files

This feature is still experimental. To enable it please supply `fluxd` with flag `--manifest-generation=true`.

## Goal

It is a common pattern to run very similar resources in separate clusters. There are various scenarios in which this is
required, the two main ones being:

* Having a staging/canary cluster and production cluster. The resources from the staging cluster are regularly promoted
  to the production cluster. In addition, there are long-term differences between the cluster resources (e.g. different
  security keys, different database endpoints etc ...).
* Federation. Different clusters run in separate regions, usually with very similar resources but different
  configurations.

The main goal of `.flux.yaml` configuration files is to help deploying similar resources/clusters:

* with minimal replication of resource definitions
* while keeping Flux neutral about the factorization technology used


## File-access behaviour in Flux

Flux performs two types of actions on raw manifest files from the Git repository:

1. Read manifest files when performing a sync operation (i.e making sure that the status of the cluster reflects what's
   in the manifest files, adjusting it if necessary)
2. Update the manifest files of [workload](https://github.com/weaveworks/flux/blob/master/site/fluxctl.md#what-is-a-workload).
   Specifically, flux can update:
    * container images, when releasing a new image version. A release can happen manually or automatically, when a new
      container image is pushed to a repository.
    * annotations, which establish the release policy of a workload (e.g. whether it should be automatically released,
      whether it should be locked from releasing, what image tags should be considered for automated releases …)

Flux can be configured to confine the scope of (1) and (2):

*   To specific (sub)directories (flag` --git-path`)
*   To a Git branch other than `master` (flag` --git-branch`)


## Abstracting out file-access: generators and updaters

Flux allows you to declare configuration files to override file operations (1) and (2), by declaring commands which
perform equivalent actions.

The configuration files are formatted in `YAML` and named `.flux.yaml`. They must be located on the Git repository
(more on this later).

A `commandUpdated` `.flux.yaml` file has the following loosely specified format:


```
version: X
commandUpdated:
  generators:
    - command: generator_command1 g1arg1 g1arg2 ...
    - command: generator_command2 g2arg2 g2arg2 ...
  updaters:
    - containerImage:
        command: containerImage_updater_command1 ciu1arg1 ciu1arg2 ...
      policy:
        command: policy_updater_command1 pu1arg1 pu1arg2 ...
    - containerImage:
        command: containerImage_updater_command2 ciu2arg1 ciu2arg2 ...
      policy:
        command: policy_updater_command2 pu2arg1 pu2arg2 ...
```


> **Note:** For a simpler approach to updates, Flux provides a `patchUpdated` configuration file variant.


The file above is versioned (in order to account for future file format changes). Current version is `1`,
which is enforced.

Also, the file contains two generators (declared in the `generators `entry), used to generate manifests and two updaters
(declared in the `updaters `entry), used to update resources in the Git repository.

The generators are meant as an alternative to Flux manifest reads (1). Each updater is split into a `containerImage`
command and a `policy` command, covering the corresponding two types of workload manifest updates mentioned in (2).

> **Note** Update commands operate on policies, rather than annotations. That is for two reasons:
>
> * It is an implementation detail for Kubernetes manifests specifically that policies are represented as annotations.
> * Some configurations (even those for Kubernetes clusters) may encode policies symbolically.

Here is a specific `.flux.yaml` example, declaring a generator and an updater using [Kustomize](https://github.com/kubernetes-sigs/kustomize)
(see [https://github.com/weaveworks/flux-kustomize-example](https://github.com/weaveworks/flux-kustomize-example)
for a complete example).


```
version: 1
commandUpdated:
  generators:
    - command: kustomize build .
  updaters:
    # use https://github.com/squaremo/kubeyaml on flux-patch.yaml
    - containerImage:
        command: >-
          cat flux-patch.yaml | 
          kubeyaml image --namespace $FLUX_WL_NS --kind $FLUX_WL_KIND --name $FLUX_WL_NAME --container $FLUX_CONTAINER --image "$FLUX_IMG:$FLUX_TAG"
          > new-flux-patch.yaml && 
          mv new-flux-patch.yaml flux-patch.yaml
      policy:
        command: >-
          cat flux-patch.yaml | 
          kubeyaml annotate --namespace $FLUX_WL_NS --kind $FLUX_WL_KIND --name $FLUX_WL_NAME "flux.weave.works/$FLUX_POLICY=$FLUX_POLICY_VALUE"
          > new-flux-patch.yaml && 
          mv new-flux-patch.yaml flux-patch.yaml
```


For every flux target path, Flux will look for a `.flux.yaml` file in the target path and all its parent directories.
If a `.flux.yaml` is found:

1. When syncing, `fluxd` will run each of the `generators`, collecting yaml manifests printed to `stdout` and applying
   them to the cluster.
2. When making a release or updating a policy, `fluxd` will run the `updaters`, which are in charge of updating the Git
   repository to reflect the required changes in workloads.
    * The `containerImage `updaters are invoked once for every container whose image requires updating.
    * The `policy` updaters are invoked once for every workload annotation which needs to be added or updated.
    * Updaters are supplied with environment variables indicating what image should be updated and what annotation to
      update (more on this later).
    * Updaters expected to modify the Git working tree in-place

    After invoking the updaters, `fluxd` will then commit and push the resulting modifications to the git repository.

3. `fluxd `will ignore any other yaml files under that path (e.g. resource manifests).

Generators and updaters are intentionally independent, in case a matching updater cannot be provided. It is hard to
create updaters for some factorization technologies (particularly Configuration-As-Code). To improve the situation,
a separate configuration file variant (`patchedUpdated`) is provided, which will be described later on.


### Execution context of commands

`generators` and `updaters` are run in a POSIX shell inside the Flux container. This means that the `command`s supplied
should be available in the [Flux container image](../docker/Dockerfile.flux). Flux currently includes `Kustomize` and
basic Unix shell tools. If the tools in the Flux image are not sufficient for your use case, you can include new tools
in your own Flux-based image or, if the tools are popular enough, Flux maintainers can add them to the Flux image
(please create an issue). In the future (once [Ephemeral containers](https://github.com/kubernetes/kubernetes/pull/59416)
are available), you will be able to specify an container image for each command.

The working directory (also known as CWD) of the `command`s executed from a `.flux.yaml` file will be set to the
target path (`--git-path` entry) used when finding that `.flux.yaml` file.

For example, when using flux with `--git-path=staging` on a git repository with this structure:


```
├── .flux.yaml
├── staging/
├──── [...]
├── production/
└──── [...]
```

The commands in `.flux.yaml `will be executed with their working directory set to `staging.`

In addition, `updaters` are provided with some environment variables:

* `FLUX_WORKLOAD`: Workload to be updated. Its format is `<namespace>:<kind>/<name>` (e.g. `default:deployment/foo`).
  For convenience (to circumvent parsing) `FLUX_WORKLOAD `is also broken down into the following environment variables:
        * `FLUX_WL_NS`
        * `FLUX_WL_KIND`
        * `FLUX_WL_NAME`
* `containerImage` updaters are provided with:
    * `FLUX_CONTAINER`: Name of the container within the workload whose image needs to be updated.
    * `FLUX_IMG`: Image name which the container needs to be updated to (e.g. `nginx`).
    * `FLUX_TAG`: Image tag which the container needs to be updated to (e.g. `1.15`).
* `policy` updaters are provided with:
    * `FLUX_POLICY`: the name of the policy to be added or updated in the workload. To make into an annotation name,
      prefix with `flux.weave.works/`
    * `FLUX_POLICY_VALUE`: value of the policy to be added or updated in the controller. If the `FLUX_POLICY_VALUE`
      environment variable is not set, it means the policy should be removed.


### Combining generators, updaters and raw manifest files

The `.flux.yaml` files support including multiple generators and updaters. Here is an example combining multiple
generators:


```
version: 1
commandUpdated:
  generators:
    - command: kustomize build .
    - command: helm template ../charts/mychart -f overrides.yaml
```


The generators/updaters will simply be executed in the presented order (top down). Flux will merge their output.

Flux supports both generated manifests and raw manifests tracked in the same repository. If Flux doesn't find a
configuration file associated to a target directory, Flux will inspect it in search for raw YAML manifest files.


### The `patchUpdated` configuration variant

We mentioned before that, while it is simple for users to provide generator commands, matching updater commands are
harder to construct. To improve the situation, Flux provides a different configuration variant: `patchUpdated`.
`patchUpdated` configurations store the modifications from Flux into a
[YAML merge patch file](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/strategic-merge-patch.md)
and implicitly apply them to the resources printed by the `generators`.

Here is an example, allowing to deploy a [Helm chart without a Tiller installation](https://jenkins-x.io/news/helm-without-tiller/)


```
version: 1
patchUpdated:
  generators:
    - command: helm template ../charts/mychart -f overrides.yaml
  patchFile: flux-patch.yaml
```

The `mergePatchUpdater` will store the modifications made by Flux in file `flux-patch.yaml` and will apply the patch to
the output of `helm template ../charts/mychart -f overrides.yaml`.

The patch file path should be relative to Flux target which matched the configuration file.

Note that the patch file will need to be kept consistent with any changes made in the generated manifests. In particular,
the patch file will be sensitive to changes in workload names, workload namespaces or workload kinds.

Lastly, here is another example using Kustomize which is much simpler than the `commandUpdated`-based example presented
earlier.

```
version: 1
commandUpdated:
  generators:
    - command: helm template ../charts/mychart -f overrides.yaml
  patchFile: flux-patch.yaml
```
