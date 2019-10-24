#!/usr/bin/env bash

function poll_until_equals() {
  local what="$1"
  local expected="$2"
  local check_cmd="$3"
  poll_until_true "$what" "[ '$expected' = \"\$( $check_cmd )\" ]"
}

function poll_until_true() {
  local what="$1"
  local check_cmd="$2"
  echo -n ">>> Waiting for $what " >&3
  retries=24
  count=0
  until eval "$check_cmd"; do
    echo -n '.' >&3
    sleep 5
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
      echo ' No more retries left!' >&3
      return 1 # fail
    fi
  done
  echo ' done' >&3
}
