#!/usr/bin/env bats

function setup() {
  load lib/env
  load lib/install
  load lib/poll
  load lib/defer
  load lib/registry
  load lib/release_image

  setup 'redis'
}

@test "Image releases" {
  image_release
}

function teardown() {
  teardown 'redis'
}
