#!/usr/bin/env bats

function setup() {
  load lib/env
  load lib/install
  load lib/poll
  load lib/defer
  load lib/registry

  kubectl create namespace "$FLUX_NAMESPACE"

  # Install the git server, allowing external access
  install_git_srv

  # Install a local registry, with some empty images to be used later in the test
  install_registry registry_result
  # shellcheck disable=SC2154
  REGISTRY_PORT="${registry_result[0]}"
  # Teardown the created port-forward to the registry.
  defer kill "${registry_result[1]}"
  # create empty images for the test
  push_empty_image "localhost:$REGISTRY_PORT" 'bitnami/ghost:3.0.2-debian-9-r3' '2020-01-20T13:53:05.47178071Z'
  push_empty_image "localhost:$REGISTRY_PORT" 'bitnami/ghost:3.1.1-debian-9-r0' '2020-02-20T13:53:05.47178071Z'
  push_empty_image "localhost:$REGISTRY_PORT" 'stefanprodan/podinfo:3.1.0' '2020-03-20T13:53:05.47178071Z'
  push_empty_image "localhost:$REGISTRY_PORT" 'stefanprodan/podinfo:3.0.5' '2020-04-20T13:53:05.47178071Z'
  REGISTRY_SERVICE_IP=$(kubectl -n "$FLUX_NAMESPACE" get service registry -o 'jsonpath={.spec.clusterIP}')

  # Finally, install Flux
  local -A template_values
  # shellcheck disable=SC2034
  template_values['REGISTRY_SERVICE_IP']="$REGISTRY_SERVICE_IP"
  # Intentionally reuse the setup from the release_image test
  install_flux_with_fluxctl '14_release_image' 'template_values'
}

@test "Fluxctl list-workloads and list-images" {
  # avoid races with the image of podinfo being automatically updated
  poll_until_true 'lock podinfo' "fluxctl --k8s-fwd-ns ${FLUX_NAMESPACE} lock --workload=demo:deployment/podinfo"

  # Wait for the registry scanner to do its magic on stefanprodan/podinfo and bitnami/ghost
  poll_until_true "stefanprodan/podinfo to be scanned" "kubectl logs -n $FLUX_NAMESPACE deploy/flux | grep -q \"component=warmer updated=stefanprodan/podinfo\"" 50
  poll_until_true "bitnami/ghost to be scanned" "kubectl logs -n $FLUX_NAMESPACE deploy/flux | grep -q \"component=warmer updated=bitnami/ghost\"" 50

  # Test fluxctl list-services
  run fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" list-workloads --namespace "${DEMO_NAMESPACE}"

  [ "$status" -eq 0 ]
  special_chars_output="$(echo "$output" | cat -vet)"
  [ "$special_chars_output" = "WORKLOAD                  CONTAINER    IMAGE                            RELEASE  POLICY$
demo:deployment/podinfo   podinfod     stefanprodan/podinfo:3.1.0       ready    automated,locked$
                          init         alpine:3.10.1                             $
demo:helmrelease/ghost    chart-image  bitnami/ghost:3.1.1-debian-9-r0           $
demo:helmrelease/mongodb  chart-image  bitnami/mongodb:4.0.13                    $
demo:helmrelease/redis    chart-image  bitnami/redis:5.0.7                       automated,locked$" ] || (
    echo "unexpected output: $special_chars_output" >&3
    exit 1
  )

  # Test fluxctl list-images
  run fluxctl --k8s-fwd-ns "${FLUX_NAMESPACE}" list-images --namespace "${DEMO_NAMESPACE}"
  [ "$status" -eq 0 ]
  special_chars_output="$(echo "$output" | cat -vet)"
  [ "$special_chars_output" = "WORKLOAD                  CONTAINER    IMAGE                  CREATED$
demo:deployment/podinfo   podinfod     stefanprodan/podinfo   $
                                       '-> 3.1.0              20 Mar 20 13:53 UTC$
                                           3.0.5              20 Apr 20 13:53 UTC$
                          init                                image data not available$
                                       '-> (untagged)         ?$
demo:helmrelease/ghost    chart-image  bitnami/ghost          $
                                       '-> 3.1.1-debian-9-r0  20 Feb 20 13:53 UTC$
                                           3.0.2-debian-9-r3  20 Jan 20 13:53 UTC$
demo:helmrelease/mongodb  chart-image                         image data not available$
                                       '-> (untagged)         ?$
demo:helmrelease/redis    chart-image                         image data not available$
                                       '-> (untagged)         ?$" ] || (
    echo "unexpected output: $special_chars_output" >&3
    exit 1
  )

}

function teardown() {
  run_deferred
  # Although the namespace delete below takes care of removing most Flux
  # elements, the global resources will not be removed without this.
  uninstall_flux_with_fluxctl
  # Removing the namespace also takes care of removing Flux and gitsrv.
  kubectl delete namespace "$FLUX_NAMESPACE"
  # Only remove the demo workloads after Flux, so that they cannot be recreated.
  kubectl delete namespace "$DEMO_NAMESPACE"
}
