# How to release Flux and the Helm operator

The release process needs to do these things:

 - create a new release on GitHub, with a tag
 - push Docker image(s) to Docker Hub
 - possibly upload the [`fluxctl` binaries](/docs/references/fluxctl.md#binary-releases) to the GitHub release
 - make sure the version is entered into the checkpoint database so that up-to-date checks report back accurate information
 - close out the GitHub milestone that was used to track the release

Much of this is automated, but it needs a human to turn the wheel.

## Overview

Flux releases use a branch named after their semver version, like `1.8.1`.

Each minor version has its own "release series" branch, from which patch releases will be put together, called e.g., `release/1.8.x`.

The CircleCI script runs builds for tags, which push Docker images and upload binaries. This is triggered by creating a release in GitHub, which will create the tag.

## Requirements

- CircleCI must have a secret environmental variable called `GITHUB_TOKEN` which is a personal access token. (This is almost certainly already set up, but mentioned here in case it needs to be reinstated.)

## Release process

**Preparing the release PR**

1. If the release is a new minor (or major) version, create a "release series" branch and push it to GitHub.

    Depending on what is to be included in the release, you may need to pick a point from which branch that is not HEAD of master. But usually, it will be HEAD of master.

2. From the release series branch, create _another_ branch for the particular release. This will be what you submit as a PR.

    For example,

    ```sh
    git checkout release/1.8.x
    git pull origin
    git checkout -b release/1.8.1
    ```

3. Put the commits you want in the release, into your branch

    If you just created a new release series branch, i.e., this is a `x.y.0` patch release, you may already have what you need, because you've just branched from master.

    If this is _not_ the first release on this branch, you will need to either merge master, or cherry-pick commits from master, to get the things you want in the release.

4. Add an entry for the new release in the `CHANGELOG.md` file

    Use `go run internal/cmd/changelog/main.go` to generate an initial version
    of the entry. Then you should refine it and edit it (i.e. adding a
    description the release, coalescing pull requests, adding pull requests in
    the right sections ... ).

    If you cherry-picked commits, remember to only mention those changes.

    To compile a list of people (GitHub usernames) to thank, you can use a script (if you have access to weaveworks/dx) or peruse the commits/PRs merged/issues since the last release. There's no exact way to do it. Be generous.

5. Consider updating the deploy manifest examples and the Helm chart.

    The example manifests are generated from template manifests at
    [pkg/install/templates](../pkg/install/templates), bump the Flux image tag
    version and run `make generate-deploy`.

    Check the changes included in the release, to see if arguments, volume
    mounts, etc., have changed.

    Read on, for how to publish a new Helm chart version.

6. Post the branch as a PR to the release series

    Push the patch release branch -- e.g., `release/1.8.1` -- to GitHub, and create a PR from it.

    > **Note:** You will need to change the branch the PR targets, from `master` to the release series, e.g., `release/1.8.x`, while creating the PR.

7. Get the PR reviewed, and merge it.

**Creating the release**

8. [Create a release in GitHub](https://github.com/fluxcd/flux/releases/new)

    Use a tag using the semver of the release (e.g. `1.8.1`).

    Copy and paste the changelog entry. You may need to remove newlines that have been inserted by your editor, so that it wraps nicely.

    Publishing the release will create the tag, and that will trigger CI to build images and binaries.

**After publishing the release**

9. Put an entry in the checkpoint database

    Add a row to the [checkpoint database](https://checkpoint-api.weave.works/admin) (or ask someone at Weaveworks to do so). This is so that the up-to-date check will report the latest available version correctly.

10. Merge the release series branch back into master, so it has the changelog entry and the updated manifests.

    You can do this by creating a new PR in GitHub -- you don't need to create any new branches, since you want to merge a branch that already exists.
    
11. Consider releasing a new version of the Helm chart, the process for this is described below.

**Bookkeeping**

12. Close the GitHub milestone relating to the release. If there are open issues or unmerged PRs in the milestone, they will need to be either reassigned to the next milestone, or (if unclear where they belong), unassigned.

## Helm chart release process

1. Create a new branch as in `chart-bump`
2. Update `appVersion` with the new Flux version in `chart/flux/Chart.yaml`
3. Bump the chart version in `chart/flux/Chart.yaml`
4. Update `image.tag` with the new Flux version in `chart/flux/values.yaml`
5. Edit `chart/flux/CHANGELOG.md` and add an entry for the release
   (including all the helm-chart PRs since the last release).
6. Create a PR from `chart-bump`
7. After the PR is merged, tag the master with the chart version `git tag
 chart-0.2.2` and push it upstream. CircleCI will identify the new tag
 and automatically add the new chart to 
 https://fluxcd.github.io/flux/index.yaml.
