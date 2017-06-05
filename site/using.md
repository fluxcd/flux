---
title: Using Weave Flux
menu_order: 40
---

All of the features of Flux are accessible from within
[Weave Cloud](https://cloud.weave.works).

However, `fluxctl` provides an equivalent API that can be used from the
command line. The `--help` for `fluxctl` is described below.

```sh
fluxctl helps you deploy your code.

Workflow:
  fluxctl list-services                                        # Which services are running?
  fluxctl list-images --service=default/foo                    # Which images are running/available?
  fluxctl release --service=default/foo --update-image=bar:v2  # Release new version.

Usage:
  fluxctl [command]

Available Commands:
  automate      Turn on automatic deployment for a service.
  deautomate    Turn off automatic deployment for a service.
  identity      Display SSH public key
  list-images   Show the deployed and available images for a service.
  list-services List services currently running on the platform.
  lock          Lock a service, so it cannot be deployed.
  release       Release a new version of a service.
  save          save service definitions to local files in platform-native format
  unlock        Unlock a service, so it can be deployed.
  version       Output the version of fluxctl

Flags:
  -t, --token string   Weave Cloud service token; you can also set the environment variable FLUX_SERVICE_TOKEN
  -u, --url string     base URL of the flux service; you can also set the environment variable FLUX_URL (default "https://cloud.weave.works/api/flux")

Use "fluxctl [command] --help" for more information about a command.

```

# What is a Service?

The `fluxctl` CLI uses the word "service" a lot. This does not represent
a Kubernetes service. Instead, it is mean in the sense that this is one
distinct resource that provides a service to others. When using
Kubernetes, this means a {deployment, service} pairing.

# Viewing Services

The first thing to do is to check whether Flux can see any running 
services. To do this, use the `list-services` subcommand:

```sh
$ fluxctl list-services                   
SERVICE             CONTAINER   IMAGE                                         RELEASE  POLICY
default/flux        fluxd       quay.io/weaveworks/fluxd:latest               ready    
                    fluxsvc     quay.io/weaveworks/fluxsvc:latest                      
default/helloworld  helloworld  quay.io/weaveworks/helloworld:master-a000001  ready    
                    sidecar     quay.io/weaveworks/sidecar:master-a000002              
default/kubernetes                                                                     
default/memcached   memcached   memcached:1.4.25                              ready    
default/nats        nats        nats:0.9.4                                    ready    
```

Note that the actual images running will depend on your cluster.

# Inspecting the Version of a Container

Once we have a list of services, we can begin to inspect which versions
of the image are running.

```sh
$ fluxctl list-images --service default/helloworld
                                SERVICE             CONTAINER   IMAGE                          CREATED
                                default/helloworld  helloworld  quay.io/weaveworks/helloworld  
                                                                |   master-9a16ff945b9e        20 Jul 16 13:19 UTC
                                                                |   master-b31c617a0fe3        20 Jul 16 13:19 UTC
                                                                |   master-a000002             12 Jul 16 17:17 UTC
                                                                '-> master-a000001             12 Jul 16 17:16 UTC
                                                    sidecar     quay.io/weaveworks/sidecar     
                                                                '-> master-a000002             23 Aug 16 10:05 UTC
                                                                    master-a000001             23 Aug 16 09:53 UTC

```

The arrows will point to the version that is currently running 
alongside a list of other versions and their timestamps.

# Releasing a Service

We can now go ahead and update a service with the `release` subcommand. 
This will check whether each service needs to be updated, and if so, 
write the new configuration to the repository.

```sh
$ fluxctl release --service=default/helloworld --user=phil --message="New version" --update-all-images
Submitting release ...
Commit pushed: 7dc025c
Applied 7dc025c61fdbbfc2c32f792ad61e6ff52cf0590a
SERVICE             STATUS   UPDATES
default/helloworld  success  helloworld: quay.io/weaveworks/helloworld:master-a000001 -> master-9a16ff945b9e

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

You can see that it is possible to record a user and a message when
releasing a service. This is handy to provide extra context in the
notifications and history.

See `fluxctl release --help` for more information.
 
# Turning on Automation

Automation can be easily controlled from within
[Weave Cloud](https://cloud.weave.works) by selecting the "Automate"
button when inspecting a service. But we can also do this from `fluxctl`
with the `automate` subcommand.

```sh
$ fluxctl automate --service=default/helloworld
Commit pushed: af4bf73
SERVICE             STATUS   UPDATES
default/helloworld  success  

$ fluxctl list-services --namespace=default         
SERVICE             CONTAINER   IMAGE                                              RELEASE  POLICY
default/flux        fluxd       quay.io/weaveworks/fluxd:latest                    ready    
                    fluxsvc     quay.io/weaveworks/fluxsvc:latest                           
default/helloworld  helloworld  quay.io/weaveworks/helloworld:master-9a16ff945b9e  ready    automated
                    sidecar     quay.io/weaveworks/sidecar:master-a000002                   
default/kubernetes                                                                          
default/memcached   memcached   memcached:1.4.25                                   ready    
default/nats        nats        nats:0.9.4                                         ready    
```

We can see that the `list-services` subcommand reports that the 
helloworld application is automated. Flux will now automatically 
deploy a new version of a service whenever one is available and 
persist the configuration to the version control system.

# Turning off Automation

Turning off automation is performed with the `deautomate` command:

```sh
 $ fluxctl deautomate --service=default/helloworld
Commit pushed: a54ef2c
SERVICE             STATUS   UPDATES
default/helloworld  success  

 $ fluxctl list-services --namespace=default      
SERVICE             CONTAINER   IMAGE                                              RELEASE  POLICY
default/flux        fluxd       quay.io/weaveworks/fluxd:latest                    ready    
                    fluxsvc     quay.io/weaveworks/fluxsvc:latest                           
default/helloworld  helloworld  quay.io/weaveworks/helloworld:master-9a16ff945b9e  ready    
                    sidecar     quay.io/weaveworks/sidecar:master-a000002                   
default/kubernetes                                                                          
default/memcached   memcached   memcached:1.4.25                                   ready    
default/nats        nats        nats:0.9.4                                         ready  
```

We can see tha the service is no longer automated.

# Locking a Service

Locking a service will prevent the manifest from being synchronised.

```sh
$ fluxctl lock --service=default/helloworld
Commit pushed: d726722
SERVICE             STATUS   UPDATES
default/helloworld  success  
```

# Unlocking a Service

Unlocking a service allows a manifest to be synchronised to the cluster.

```sh
$ fluxctl unlock --service=default/helloworld
Commit pushed: 708b63a
SERVICE             STATUS   UPDATES
default/helloworld  success  
```