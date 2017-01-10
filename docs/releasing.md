# How to make a release of flux

This process will create a new tagged release of flux, push dockerfiles and upload the `fluxctl` binary to GitHub releases.

## Requirements
- Circle CI must have a secret environmental variable called `GITHUB_TOKEN` which is a personal access token.

## Release process

1. Alter and commit the /CHANGELOG.md file to signify what has changed in this version.
2. Create a new release: https://github.com/weaveworks/flux/releases/new
4. Fill in the version number for the name and tag. The version number should be semantic in the form `v1.2.3`.
5. Fill in the Description field (possibly a copy paste from the CHANGELOG.md)
6. Click "Publish release"

Circle will then run the build and upload the built binaries to the "Downloads" section of the release.

Circle will also create a new tag called "latest_release" which points to the same commit and contains the same binaries as those just built.

## Outputs

The most recent binaries are always available at: https://github.com/weaveworks/flux/releases/latest
