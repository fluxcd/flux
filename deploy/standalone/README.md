# Standalone Flux

Flux comes in two parts: the service, which serves the API for the
command-line client to use, and the daemon, which carries out some
tasks on behalf of the service.

Usually you would run the daemon yourself, and use the service that
runs in Weave Cloud. But you can also run both of these in your own
cluster.

## Example deployment manifest

The file `flux-deployment.yaml` contains a Kubernetes deployment
configuration that runs the Flux service and the Flux daemon in a
single pod.

In a standalone deployment, the manifest doesn't need any
customisation. You can just create it:

```
kubectl create -f flux-deployment.yaml
```

To make the pod accessible to the command-line client `fluxctl`, you
can create a service for Flux. The example in `flux-service.yaml`
exposes the service as a
[`NodePort`](http://kubernetes.io/docs/user-guide/services/#type-nodeport).

```
kubectl create -f flux-service.yaml
```

Now you need to tell `fluxctl` where to find the service. If you're
using minikube, say, you can get the IP address of the host, and the
port, with

```
$ flux_host=$(minikube ip)
$ flux_port=$(kubectl get service fluxsvc --template '{{ index .spec.ports 0 "nodePort" }}')
$ export FLUX_URL=http://$flux_host:$flux_port
```

At this point you can see if it's all running by doing:

```
fluxctl list-services
```
