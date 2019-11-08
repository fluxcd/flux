#!/usr/bin/env bash

function fill_in_place_recursively() {
  local -n key_values=$1 # pass an associate array as a nameref
  local target_directory=${2:-.}
  (# use a subshell to expose key-values as variables for envsubst to use
    for key in "${!key_values[@]}"; do
      export "$key"="${key_values[$key]}"
    done
    # Use find with zero-ended strings and read to avoid problems
    # with spaces in paths
    while IFS= read -r -d '' file; do
      # Use a command group to ensure "$file" is not
      # deleted before being written to.
      # shellcheck disable=SC2094
      {
        rm "$file"
        envsubst > "$file"
      } < "$file"
    done < <(find "$target_directory" -type f -print0)
  )
}
