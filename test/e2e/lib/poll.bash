#!/usr/bin/env bash

function poll_until_equals() {
  local what="$1"
  local expected="$2"
  local check_cmd="$3"
  local retries="$4"
  local wait_period="$5"
  poll_until_true "$what" "[ '$expected' = \"\$( $check_cmd )\" ]" "$retries" "$wait_period"
}

function poll_until_true() {
  local what="$1"
  local check_cmd="$2"
  # timeout after $retries * $wait_period seconds
  local retries=${3:-24}
  local wait_period=${4:-5}
  echo -n ">>> Waiting for $what " >&3
  count=0
  until eval "$check_cmd"; do
    echo -n '.' >&3
    sleep "$wait_period"
    count=$((count + 1))
    if [[ ${count} -eq ${retries} ]]; then
      echo ' No more retries left!' >&3
      return 1 # fail
    fi
  done
  echo ' done' >&3
}
