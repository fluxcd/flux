# Manifest generation through `.flux.yaml` configuration files

This feature lets you generate Kubernetes manifests with a program,
instead of having to include them in your git repo as YAML files. For
example, you can use `kustomize` to patch a common set of resources to
suit a particular environment.

## Setting manifest generation up

Manifest generation is controlled by the flags given to `fluxd`, and
`.flux.yaml` files in your git repo.

To enable it, pass the command-line flag `--manifest-generation=true`
to `fluxd`.

For each path supplied with the flag `--git-path` (or `.` if none are
supplied),

 - if a `.flux.yaml` file is found, it is used **instead** of looking
   for YAML files;

 - if no `.flux.yaml` file is found, the usual behaviour of looking
   for YAML files is used.

The manifests from all the `--git-path` values -- read from YAML files
or generated -- are combined before applying to the cluster. If
duplicates are detected, an error is logged and the sync is abandoned.

A `.flux.yaml` file can be _at_ the path mentioned, or in a directory
_above_ it in the git repo. Here are some examples:

```sh
# show normal files _and_ .flux.yaml
flux-kustomization-example$ tree -a -P .flux.yaml -P '*' -I .git
.
├── base
│   ├── demo-ns.yaml
│   ├── kustomization.yaml
│   ├── podinfo-dep.yaml
│   ├── podinfo-hpa.yaml
│   └── podinfo-svc.yaml
├── .flux.yaml
├── LICENSE
├── production
│   ├── flux-patch.yaml
│   ├── kustomization.yaml
│   └── replicas-patch.yaml
├── README.md
└── staging
    ├── flux-patch.yaml
    └── kustomization.yaml
```

In this case, say you started `fluxd` with `--git-path=staging`, it
would find `.flux.yaml` in the top directory and use that to generate
manifests.  The files and directories (if there were any) in
`staging/` are ignored, in favour of the instructions given in the
`.flux.yaml` file.

In this modified example,

```sh
flux-kustomization-example$ tree -a -P .flux.yaml -P '*' -I .git
.
├── base
│   ├── demo-ns.yaml
│   ├── kustomization.yaml
│   ├── podinfo-dep.yaml
│   ├── podinfo-hpa.yaml
│   └── podinfo-svc.yaml
├── LICENSE
├── production
│   ├── flux-patch.yaml
│   ├── .flux.yaml
│   ├── kustomization.yaml
│   └── replicas-patch.yaml
├── README.md
└── staging
    ├── flux-patch.yaml
    └── kustomization.yaml
```

… the `.flux.yaml` file is under `production/`, so it will _only_
take effect for `--git-path=production`.

Using `--git-path=staging` would **not produce a usable
configuration**, because without an applicable `.flux.yaml`, the files
under `staging/` would be treated as plain Kubernetes manifests, which
they are plainly not.

Note also that the configuration file would **not** take effect for
`--git-path=.` (i.e., the top directory), because manifest generation
will not look in subdirectories for a `.flux.yaml` file.

## How to construct a .flux.yaml file

`.flux.yaml` files come in two varieties: "patch-updated", and
"command-updated". These refer to the way in which automated updates
are applied to files in the repo:

 - when patch-updated, fluxd will keep updates in its own patch file,
   which it applies to the generated manifests before applying to the
   cluster;
 - when command-updated, you must supply commands to update the
   appropriate file or files.

Patch-updated will work with any kind of manifest generation, because
the patch is entirely managed by `fluxd` and applied post-hoc to the
manifests.

Command-updated is more general, but since you need to supply your own
programs to find and update the right file, it is likely to be a lot
more work.

Both patch-updated and command-updated configurations have the same
way of specifying how to generate manifests, and differ only in how
updates are recorded.

### Generator configuration

Here is an example of a `.flux.yaml`:

```yaml
version: 1 # must be `1`
patchUpdated:
  generators:
  - command: kustomize build .
  patchFile: flux-patch.yaml
```

The `generators` field is an array of commands, all of which will be
run in turn. Each command is expected to print a YAML stream to its
stdout. The streams are concatenated and parsed as one big YAML
stream, before being applied.

Much of the time, it will only be necessary to supply one command to
be run.

The commands will be run with the `--git-path` being processed as a
working directory -- which is not necessarily the same directory in
which the `.flux.yaml` file was found. [See below](#execution-context)
for more details on the execution context in which commands are run.

### Using patch-updated configuration

A patch-updated configuration generates manifests using commands, and
records updates in a patch file.

For example, when an automated image upgrade is run, fluxd will do
this:

 1. run the generator commands and parse the manifests;
 2. find the manifest that needs to be updated, and calculate the
    patch to it that performs the update;
 3. record that patch in the patch file.

When syncing, fluxd will generate the manifests as usual, then apply
all the patches that have been recorded in the patch file.

This is how a patch-updated `.flux.yaml` looks in general:

```yaml
version: 1
patchUpdated:
  generators:
  - command: generator_command
  patchFile: path/to/patch.yaml
```

The `generators` field is explained just above. The `patchFile` field
gives a path, relative to the `--git-path` value, in which to record
patches. `fluxd` will create the file if necessary, and commit any
changes it makes to git.

### Using command-updated configuration

A command-updated configuration generates manifests in the same way,
but records changes by running commands as given in the `.flux.yaml`.

This is how a command-updated `.flux.yaml` looks in general:

```yaml
version: 1
commandUpdated:
  generators:
  - command: generator_command
  updaters:
  - containerImage:
      command: image_updater_program
    policy:
      command: policy_updater_program
```

The `updaters` section is particular to command-updated
configuration. It contains an array of updaters, each of which gives a
command for updating container images, and a command for updating
policies (policy controls how automated updates should be applied to a
resource; these appear as annotations in generated manifests).

When asked to update a resource, fluxd will run execute the
appropriate variety of command for *each* entry in `updaters:`. For
example, when updating an image, it will execute the command under
`containerImage`, for each updater entry, in turn.

Usually updates come in batches -- e.g., updating the same container
image in several resources -- so the commands will likely be run
several times.

### Execution context of commands

`generators` and `updaters` are run in a POSIX shell inside the fluxd
container. This means that the executables mentioned in commands must
be available in the [Flux container
image](https://github.com/fluxcd/flux/blob/master/docker/Dockerfile.flux).

Flux currently includes `kustomize` and basic Unix shell tools. If the
tools in the Flux image are not sufficient for your use case, you can
include new tools in your own Flux-based image or, if the tools are
popular enough, Flux maintainers can add them to the Flux image
(please create an issue). In the future it may be possibly to specify
an container image for each command.

The working directory (also known as CWD) of the `command`s executed
from a `.flux.yaml` file will be set to the target path, i.e., the
`--git-path` entry.

For example, when using flux with `--git-path=staging` on a git
repository with this structure:

```sh
├── .flux.yaml
├── staging/
├──── [...]
├── production/
└──── [...]
```

The commands in `.flux.yaml` will be executed with their working
directory set to `staging`.

In addition, `updaters` are given arguments via environment variables:

 * `FLUX_WORKLOAD`: the workload to be updated. Its format is
  `<namespace>:<kind>/<name>` (e.g. `default:deployment/foo`). For
  convenience (to circumvent parsing) `FLUX_WORKLOAD` is also broken
  down into the following environment variables:
  * `FLUX_WL_NS`
  * `FLUX_WL_KIND`
  * `FLUX_WL_NAME`

 * `containerImage` updaters are provided with:
   * `FLUX_CONTAINER`: Name of the container within the workload whose image needs to be updated.
   * `FLUX_IMG`: Image name which the container needs to be updated to (e.g. `nginx`).
   * `FLUX_TAG`: Image tag which the container needs to be updated to (e.g. `1.15`).

 * `policy` updaters are provided with:
   * `FLUX_POLICY`: the name of the policy to be added or updated in
     the workload. To make into an annotation name, prefix with
     `fluxcd.io/`
   * `FLUX_POLICY_VALUE`: value of the policy to be added or updated
     in the controller. If the `FLUX_POLICY_VALUE` environment
     variable is not set, it means the policy should be removed.

Please note that the default timeout for sync commands is set to one
minute. If you run into errors like `error executing generator
command: context deadline exceeded`, you can increase the timeout with
the `--sync-timeout` fluxd command flag or the `sync.timeout` Helm
chart option.
