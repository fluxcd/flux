#!/usr/bin/env bash

# This lets you call `defer` to record an action to do later;
# `run_deferred` should be called in an EXIT trap, either explicitly:
#
#    trap run_deferred EXIT
#
# or when using with tests, by calling it in the teardown function
# (which bats will arrange to run).

declare -a on_exit_items

function run_deferred() {
  if [ "${#on_exit_items[@]}" -gt 0 ]; then
    echo -e '\nRunning deferred items, please do not interrupt until they are done:'
  fi
  for I in "${on_exit_items[@]}"; do
    echo "deferred: ${I}"
    eval "${I}"
  done
}

function defer() {
  on_exit_items=("$*" "${on_exit_items[@]}")
}
