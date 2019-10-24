#!/usr/bin/env bash

declare -a on_exit_items

function on_exit() {
  if [ "${#on_exit_items[@]}" -gt 0 ]; then
    echo -e '\nRunning deferred items, please do not interrupt until they are done:'
  fi
  for I in "${on_exit_items[@]}"; do
      echo "deferred: ${I}"
      eval "${I}"
  done
}

trap on_exit EXIT

function defer() {
  on_exit_items=("$*" "${on_exit_items[@]}")
}