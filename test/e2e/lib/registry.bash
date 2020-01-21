#!/usr/bin/env bash

# shellcheck disable=SC1090
source "${E2E_DIR}/lib/defer.bash"
# shellcheck disable=SC1090
source "${E2E_DIR}/lib/template.bash"

# pushes an empty image (layerless) to a given registry
function push_empty_image() {
  local registry_host=$1
  local image_name_and_tag=$2
  local creation_time=$3 # Format: 2020-01-20T13:53:05.47178071Z

  image_dir=$(mktemp -d)
  defer rm -rf "$image_dir"
  cp "${E2E_DIR}/fixtures/crane_empty_img_tmpl/"* "${image_dir}/"
  local -A template_values
  # shellcheck disable=SC2034
  template_values['CREATION_TIME']="$creation_time"
  fill_in_place_recursively 'template_values' "$image_dir"

  tar -cf "${image_dir}/image.tar" -C "$image_dir" manifest.json config.json
  crane push "${image_dir}/image.tar" "${registry_host}/${image_name_and_tag}"
}
