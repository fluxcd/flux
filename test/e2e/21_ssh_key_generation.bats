#!/usr/bin/env bats

function setup() {
  load lib/env
  load lib/install
  load lib/defer

  kubectl create namespace "$FLUX_NAMESPACE"

  # Installing Flux with the defaults _without_ installing the git
  # server (which generates an SSH key used by the server and Flux)
  # will cause Flux to generate an SSH key.
  install_flux_with_fluxctl
}

@test "SSH key is generated" {
  run fluxctl identity --k8s-fwd-ns "$FLUX_NAMESPACE"
  [ "$status" -eq 0 ]
  [ "$output" != "" ]

  fingerprint=$(echo "$output" | ssh-keygen -E md5 -lf - | awk '{ print $2 }')
  [ "$fingerprint" = "MD5:$(fluxctl identity -l --k8s-fwd-ns "$FLUX_NAMESPACE")" ]
}

function teardown() {
  run_deferred
  # Although the namespace delete below takes care of removing most Flux
  # elements, the global resources will not be removed without this.
  uninstall_flux_with_fluxctl
  # Removing the namespace also takes care of removing Flux and gitsrv.
  kubectl delete namespace "$FLUX_NAMESPACE"
}
