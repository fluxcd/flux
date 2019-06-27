# Flux Helm Repository

Flux is a tool that automatically ensures that the state of a cluster matches the config in git. 
It uses an operator in the cluster to trigger deployments inside Kubernetes, which means you don't need a separate CD tool. 
It monitors all relevant image repositories, detects new images, triggers deployments and updates the desired running
configuration based on that (and a configurable policy).

Flux Helm chart version: 0.10.2

## Usage

Install instructions can be found at [chart/flux/README.md](https://github.com/weaveworks/flux/blob/master/chart/flux/README.md)




