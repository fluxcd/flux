# Deploying Fluxy to Kubernetes

You will need to build or load the weaveworks/fluxy image into the
Docker daemon, since the deployment does not attempt to pull the image
from a registry. If you're using
[minikube](https://github.com/kubernetes/minikube) to try things
locally, for example, you can do

```
eval $(minikube docker-env)
make clean build
```

which will build the image in minikube's Docker daemon, thus making it
available to Kubernetes.

The file `fluxy-deployment.yaml` contains a Kubernetes deployment
configuration that runs the latest image of Fluxy.

    kubectl create -f fluxy-deployment.yaml

To make the pod accessible to `fluxctl`, you can port forward:

    kubectl port-forward $(kubectl get pods | grep fluxy | awk '{print $1}') 3030:3030

This will work with the default settings of `fluxctl`, and is
especially handy with minikube.

To force Kubernetes to run the latest image, kill the pod:

```
kubectl get pods | grep fluxy | awk '{ print $1 }' | xargs kubectl delete pod
```
