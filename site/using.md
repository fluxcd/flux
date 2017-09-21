---
title: Using Weave Flux
menu_order: 40
---

All of the features of Flux are accessible from within
[Weave Cloud](https://cloud.weave.works).

However, `fluxctl` provides an equivalent API that can be used from
the command line. The `--help` for `fluxctl` is described below.

```sh
fluxctl helps you deploy your code.

Workflow:
  fluxctl list-services                                        # Which services are running?
  fluxctl list-images --service=default/foo                    # Which images are running/available?
  fluxctl release --service=default/foo --update-image=bar:v2  # Release new version.

Usage:
  fluxctl [command]

Available Commands:

Having no deployment side effect

  version       Output the version of fluxctl
  identity      Display SSH public key
  list-images   Show the deployed and available images for a service.
  list-services List services currently running on the platform.
  save          save service definitions to local files in platform-native format

With side effect

  automate      Turn on automatic deployment for a service.
  deautomate    Turn off automatic deployment for a service.
  lock          Lock a service, so it cannot be deployed.
  release       Release a new version of a service.
  unlock        Unlock a service, so it can be deployed.

Flags:
  -t, --token string   Weave Cloud service token; you can also set the environment variable FLUX_SERVICE_TOKEN
  -u, --url string     base URL of the flux service; you can also set the environment variable FLUX_URL (default "https://cloud.weave.works/api/flux")

Use "fluxctl [command] --help" for more information about a command.

```

# What is a Service?

The `fluxctl` CLI uses the word "service" a lot. This does not represent
a Kubernetes service. Instead, it is meant in the sense that this is one
distinct resource that provides a service to others. In the
Kubernetes sense, this means a {deployment, service} pairing.

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
deploy a new version of a service whenever one is available and commit
the new configuration to the version control system.

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

We can see that the service is no longer automated.

# Locking a Service

Locking a service will stop manual or automated releases to that
service. Changes made in the file will still be synced.

```sh
$ fluxctl lock --service=default/helloworld
Commit pushed: d726722
SERVICE             STATUS   UPDATES
default/helloworld  success  
```

# Unlocking a Service

Unlocking a service allows it to have manual or automated releases
(again).

```sh
$ fluxctl unlock --service=default/helloworld
Commit pushed: 708b63a
SERVICE             STATUS   UPDATES
default/helloworld  success  
```

# Recording user and message with the triggered action

Issuing a deployment change results in a version control change/git commit, keeping the
history of the actions. Flux daemon can be started with several flags that impact the commit
information:

| flag              | purpose                       | default |
|-------------------|-------------------------------|------------|
| git-user          | committer name                | Weave Flux |
| git-email         | committer name                | support@weave.works |
| set-git-author    | override of the commit author | false |

Actions triggered by a user through the Weave Cloud UI or the CLI `fluxctl`
tool, can have the commit author information customized. This is handy for providing extra context in the
notifications and history. Whether the customization is possible, depends on the Flux daemon (fluxd)
`set-git-author` flag. If set, the commit author will be customized in the following way:

## Actions triggered through Weave Cloud

Weave Cloud UI sends user parameter, value of which is the username (email) of the user logged into
Weave Cloud.

## Actions triggered through `fluxctl`

`fluxctl` provides the following flags for the message and author customization:

  -m, --message string      message associated with the action
      --user    string      user who triggered the action

Commit customization

    1. Commit message

       fluxctl --message="Message providing more context for the action" .....

    2. Committer

        Committer information can be overriden with the appropriate fluxd flags:

        --git-user
        --git-email

        See [site/daemon.md] for more information.

    3. Commit author

        The default for the author is the committer information, which can be overriden,
        in the following manner:

        a) Default override uses user's git configuration, ie user.name
           and user.email (.gitconfig) to set the commit author.
           If the user has neither user.name nor for
           user.email set up, the committer information will be used. If only one
           is set up, that will be used.

        b) This can be further overriden by the use of the fluxctl --user flag.

        Examples

        a) fluxctl --user="Jane Doe <jane@doe.com>" ......
            This will always succeed as git expects a new author in the format
            "some_string <some_other_string>".

        b) fluxctl --user="Jane Doe" .......
            This form will succeed if there is already a repo commit, done by
            Jane Doe.

        c) fluxctl --user="jane@doe.com" .......
            This form will succeed if there is already a repo commit, done by
            jane@doe.com.

## Errors due to author customization

In case of no prior commit by the specified author, an error will be reported
for b) and c):

git commit: fatal: --author 'unknown' is not 'Name <email>' and matches
no existing author

