#!/usr/bin/env bash

function fill_in_place_recursively() {
  local -n key_values=$1 # pass an associate array as a nameref
  local target_directory=${2:-.}
  (# use a subshell to expose key-values as variables for envsubst to use
    for key in "${!key_values[@]}"; do
      export "$key"="${key_values[$key]}"
    done
    while IFS= read -r -d '' file; do
      # shellcheck disable=SC2094
      {
        rm "$file"
        envsubst > "$file"
      } < "$file"
    done < <(find "$target_directory" -type f -print0)
  )
}
