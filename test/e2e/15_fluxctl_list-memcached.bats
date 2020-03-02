#!/usr/bin/env bats

function setup() {
  load lib/env
  load lib/install
  load lib/poll
  load lib/defer
  load lib/registry
  load lib/fluxctl_list

  setup 'memcached'
  warmup
}

@test "Fluxctl list-workloads" {
  # Test fluxctl list-services
  run fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" list-workloads --namespace "${DEMO_NAMESPACE}"
  list_workloads "${status}" "${output}"
}

@test "Fluxctl list-images" {
  # Test fluxctl list-images
  run fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" list-images --namespace "${DEMO_NAMESPACE}"
  list_images "${status}" "${output}"
}

function teardown() {
  teardown 'memcached'
}
