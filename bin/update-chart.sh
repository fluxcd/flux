#!/bin/bash

set -v
set -e
set -u
set -o pipefail

scratch=$(mktemp -d -t flux-chart.XXXXXXXXXX)
function cleanup {
    rm -rf "$scratch"
}
trap cleanup EXIT

REV="$1"
if [ -z "$REV" ]; then
    echo "Expected revision as sole argument" >&2
    exit 1
fi

git clone -b "$REV" https://github.com/weaveworks/flux "$scratch"
helm package "$scratch/chart/flux/"
helm repo index . --url https://weaveworks.github.io/flux --merge index.yaml
