---
title: Using Weave Flux
menu_order: 30
---

On a day-to-day basis, Flux is designed to be used through Weave Cloud.

However, when setting up and when requiring more information than the
UI provides, `fluxctl` provides a comprehensive API. The `--help` for
 `fluxctl` is described below. 

```sh
Workflow:
  fluxctl list-services                                        # Which services are running?
  fluxctl list-images --service=default/foo                    # Which images are running/available?
  fluxctl release --service=default/foo --update-image=bar:v2  # Release new version.
  fluxctl history --service=default/foo                        # Review what happened

Usage:
  fluxctl [command]

Available Commands:
  automate      Turn on automatic deployment for a service.
  check-release Check the status of a release.
  deautomate    Turn off automatic deployment for a service.
  get-config    display configuration values for an instance
  history       Show the history of a service or all services
  list-images   Show the deployed and available images for a service.
  list-services List services currently running on the platform.
  lock          Lock a service, so it cannot be deployed.
  release       Release a new version of a service.
  set-config    set configuration values for an instance
  status        display current system status
  unlock        Unlock a service, so it can be deployed.
  version       Output the version of fluxctl
```

# Typical Usage

## Fluxctl Setup

You need to tell `fluxctl` where to find the Flux service. If you're
using minikube, say, you can get the IP address of the host, and the
port, with

```
$ flux_host=$(minikube ip)
$ flux_port=$(kubectl get service fluxsvc --template '{{ index .spec.ports 0 "nodePort" }}')
$ export FLUX_URL=http://$flux_host:$flux_port/api/flux
```

## Viewing Services

The first thing to do is to check whether Flux can see any running 
services. To do this, use the `list-services` subcommand:

```sh
$ fluxctl list-services                   
SERVICE                           CONTAINER             IMAGE                                                       RELEASE  POLICY
default/fluxsvc                   fluxd                 weaveworks/fluxd:test                                                
                                  fluxsvc               weaveworks/fluxsvc:test                                              
default/kubernetes                                                                                                           
default/memcached                 memcached             memcached:1.4.25                                                     
kube-system/kube-dns              kubedns               gcr.io/google_containers/kubedns-amd64:1.9                           
                                  dnsmasq               gcr.io/google_containers/kube-dnsmasq-amd64:1.4                      
                                  healthz               gcr.io/google_containers/exechealthz-amd64:1.2                       
kube-system/kubernetes-dashboard  kubernetes-dashboard  gcr.io/google_containers/kubernetes-dashboard-amd64:v1.5.1      
```

Note that the actual images running will depend on your cluster.

## Inspecting the Version of a Container

Once we have a list of services, we can begin to inspect which versions
of the image are running.

```sh
$ fluxctl list-images --service kube-system/kube-dns
SERVICE               CONTAINER  IMAGE                                        CREATED
kube-system/kube-dns  kubedns    gcr.io/google_containers/kubedns-amd64       
                                 '-> 1.9                                      19 Nov 16 00:06 UTC
                                     1.8                                      29 Sep 16 16:43 UTC
                                     1.7                                      24 Aug 16 21:39 UTC
                                     1.6-test                                 02 Jul 16 02:05 UTC
                                     1.6                                      29 Jun 16 22:05 UTC
                                     1.5                                      24 Jun 16 06:26 UTC
                                     1.4                                      22 Jun 16 19:41 UTC
                                     1.3                                      04 Jun 16 03:29 UTC
                                     1.2                                      02 Jun 16 22:12 UTC
                                     1.2.test                                 28 May 16 01:19 UTC
                      dnsmasq    gcr.io/google_containers/kube-dnsmasq-amd64  
                                 :                                            
                                 '-> 1.4                                      29 Sep 16 16:26 UTC
                      healthz    gcr.io/google_containers/exechealthz-amd64   
                                 :                                            
                                 '-> 1.2                                      22 Sep 16 22:25 UTC

```

The arrows will point to the version that is currently running 
alongside a list of other versions and their timestamps.

## Deploy a Test Service

In order to use Flux, we need a service that we can deploy.

Fork the [flux-example](https://github.com/weaveworks/flux-example)
repository to your own account on github.

You can run the helloworld service by creating the deployment and
service resources given as files in that repo:

```
$ cd flux-example
$ kubectl create -f helloworld-deploy.yaml -f helloworld-svc.yaml
```

## Flux configuration

In order to perform most actions in Flux, some configuration is 
required. Obtain a blank copy of the current configuration with the 
`get-config` sub-command:

```sh
$ fluxctl get-config > flux.conf                                       
```

Now edit the file `flux.conf` -- it'll look like this:

```yaml
git:
  URL: ""
  path: ""
  branch: ""
  key: ""
slack:
  hookURL: ""
  username: ""
registry:
  auths: {}
```

### Git

Alter the git settings to point to a Git repository that you own.
Flux will push kubernetest manifests that represent the state of the 
cluster to Git. To do this it requires an SSH deploy key. 
To generate an insert a key manually, follow the
instructions 
[on the Github website](https://developer.github.com/guides/managing-deploy-keys/#deploy-keys).

Ensure that generated keys have write access (i.e. do not check 
read-only).

Once generated, add the private key to the `key` field in the 
configuration. When you perform the next `get-config` it
will only display the public version of the key.

Be careful about the formatting of the deploy key.
Any extra whitespace may invalidate the key.

### Slack

For slack integration, add an "Incoming Webhoook" to slack, then copy
the webhook URL to the Flux settings. You can also optionally 
override the username used by slack when posting messages.

## Docker

The registry settings are if you need to connect to a private container 
registry. These are the settings that are normally found in 
`~/.docker/config.json` (i.e. they are just base64-encoded 
`<username>:<password>`).

(NB the key is a URL, and will usually have to be quoted as it is above.)

### Full example

Below is a complete example:

```yaml
git:
  URL: git@github.com:squaremo/flux-example
  path: 
  branch: master
  key: |
         -----BEGIN RSA PRIVATE KEY-----
         ZNsnTooXXGagxg5a3vqsGPgoHH1KvqE5my+v7uYhRxbHi5uaTNEWnD46ci06PyBz
         zSS6I+zgkdsQk7Pj2DNNzBS6n08gl8OJX073JgKPqlfqDSxmZ37XWdGMlkeIuS21
         nwli0jsXVMKO7LYl+b5a0N5ia9cqUDEut1eeKN+hwDbZeYdT/oGBsNFgBRTvgQhK
         ... contents of private key ...
         -----END RSA PRIVATE KEY-----
slack:
  hookURL: "https://hooks.slack.com/services/S2KDHXXXX/B323PXXXX/82aP..."
  username: "custom-username-bot"
registry:
  auths:
    'https://index.docker.io/v1/':
      auth: "dXNlcm5h..."
```

Note the use of `|` to have a multiline string value for the key; all
the lines must be indented if you use that.

## Structuring the configuration repository

The repository that holds cluster state should be structured in a 
particular way.

Flux supports pushing to a single repository only.
Multiple applications on the same cluster may be supported by multiple
instances of Flux.
Each Kubernetes component should have its own file. 
Files may be separated into subfolders.

A simple example can be 
[found here](https://github.com/weaveworks/flux-example). A slightly
more complex example can be found in the 
[Microservices Demo](https://github.com/microservices-demo/microservices-demo/tree/master/deploy/kubernetes/manifests)
reference architecture.

## Releasing a Service

We can now go ahead and update a service with the `release` subcommand. 
This will check whether each service needs to be updated, and if so, 
write the new configuration to the repository.

```sh
$ fluxctl list-images --service default/helloworld 
SERVICE             CONTAINER   IMAGE                          CREATED
default/helloworld  helloworld  quay.io/weaveworks/helloworld  
                                |   master-9a16ff945b9e        20 Jul 16 13:19 UTC
                                |   master-b31c617a0fe3        20 Jul 16 13:19 UTC
                                '-> master-a000002             12 Jul 16 17:17 UTC
                                    master-a000001             12 Jul 16 17:16 UTC
                    sidecar     quay.io/weaveworks/sidecar     
                                '-> master-a000002             23 Aug 16 10:05 UTC
                                    master-a000001             23 Aug 16 09:53 UTC

$ fluxctl release --service=default/helloworld --update-all-images
Submitting release job...
Release job submitted, ID c5e39f46-171d-349e-ac43-fbbc17018848
Status: Complete.

Here's what happened:
 1) Queued.
 2) Calculating release actions.
 3) Release latest images to default/helloworld
 4) Service default/helloworld image quay.io/weaveworks/sidecar:master-a000002 is already the latest one; skipping.
 5) Clone the config repo.
 6) Clone OK.
 7) Update 1 images(s) in the resource definition file for default/helloworld: helloworld (quay.io/weaveworks/helloworld:master-a000002 -> quay.io/weaveworks/helloworld:master-9a16ff945b9e).
 8) Update pod controller OK.
 9) Commit and push the config repo.
 10) Pushed commit: Release latest images to default/helloworld
 11) Release 1 service(s): default/helloworld.
Took 8.306013228s

$ fluxctl list-images --service default/helloworld    
SERVICE             CONTAINER   IMAGE                          CREATED
default/helloworld  helloworld  quay.io/weaveworks/helloworld  
                                '-> master-9a16ff945b9e        20 Jul 16 13:19 UTC
                                    master-b31c617a0fe3        20 Jul 16 13:19 UTC
                                    master-a000002             12 Jul 16 17:17 UTC
                                    master-a000001             12 Jul 16 17:16 UTC
                    sidecar     quay.io/weaveworks/sidecar     
                                '-> master-a000002             23 Aug 16 10:05 UTC
                                    master-a000001             23 Aug 16 09:53 UTC

```

See `fluxctl release --help` for more information.
 
## Turning on Automation

Automation can be easily controlled from within
[Weave Cloud](https://cloud.weave.works/instances) by selecting the 
"Automate" button when inspecting a service. But we can also 
do this from `fluxctl` with the `automate` subcommand.

```sh
$ fluxctl automate --service=default/helloworld

$ fluxctl list-services                                                 
SERVICE                           CONTAINER             IMAGE                                                       RELEASE  POLICY
default/fluxsvc                   fluxd                 weaveworks/fluxd:test                                                
                                  fluxsvc               weaveworks/fluxsvc:test                                              
default/helloworld                helloworld            quay.io/weaveworks/helloworld:master-9a16ff945b9e                    automated
                                  sidecar               quay.io/weaveworks/sidecar:master-a000002                            
default/kubernetes                                                                                                           
default/memcached                 memcached             memcached:1.4.25                                                     
kube-system/kube-dns              kubedns               gcr.io/google_containers/kubedns-amd64:1.9                           
                                  dnsmasq               gcr.io/google_containers/kube-dnsmasq-amd64:1.4                      
                                  healthz               gcr.io/google_containers/exechealthz-amd64:1.2                       
kube-system/kubernetes-dashboard  kubernetes-dashboard  gcr.io/google_containers/kubernetes-dashboard-amd64:v1.5.1           

```

We can see that the `list-services` subcommand reports that the 
helloworld application is automated. Flux will now automatically 
deploy a new version of a service whenever one is available and 
persist the configuration to the version control system.
