# Get Started Developing Flux

This guide shows a workflow for making a small (actually, tiny) change to Flux, building and testing that change locally.

> ### TL;DR
>
> From a very high level, there are at least 3 ways you can develop on Flux once you have your environment set up:
> 1. The "minimalist" approach (only requires and `kubectl`):
>    1. `make`
>    1. copy the specific image tag (e.g. `docker.io/fluxcd/flux:master-a86167e4`) for what you just built and paste it into `/deploy/flux-deployment.yaml` as the image you're targeting to deploy
>    1. deploy the resources in `/develop/*.yaml` manually with `kubectl apply`
>    1. make a change to the code
>    1. see your code changes have been deployed
>    1. repeat
> 1. Use `freshpod` to deploy changes to the `/deploy` directory resources:
>    1. `make`
>    1. make a change to the code
>    1. see your code changes have been deployed
>    1. repeat
> 1. Remote cluster development approach:
>    1. ensure local kubectl access to a remote kubernetes cluster.
>    1. have an available local memcached instance.
>    1. make a change to the code
>    1. ```bash
>          go run cmd/fluxd/main.go \
>           --memcached-hostname localhost  \
>           --memcached-port 11211 \
>           --memcached-service "" \
>           --git-url git@github.com:weaveworks/flux-get-started \
>           --k8s-in-cluster=false
>       ```
>    1. repeat
> 1. Use `helm` and `skaffold` together to deploy changes to the Flux helm chart.
>    1. `make`
>    1. make a change to the code
>    1. see your code changes have been deployed
>    1. repeat
>
> This guide covers approaches 1 and 2 using `minikube`. `freshpod` is superseded by `Skaffold` and is generally the future.  That said, `freshpod` is very simple to use and reason about (and is still well supported by `minikube`) which is why it's used in this guide.

## Run `flux-getting-started`

We're going to make some changes soon enough, but just to get a good baseline please follow the [Getting Started](site/get-started.md) guide and run the `flux-getting-started` repo through its normal paces.

Now that we know everything is working with `flux-getting-started`, we're going to try and do nearly the same thing as `flux-getting-started`, except instead of using official releases of flux, we're going to build and run what we have locally.

## Prepare your Environment

1. Install the prerequisites.  This guide is written from running Linux, but the same instructions will generally apply to OSX.  Although everything you need has been known to work independently in Windows from time to time, results may vary.
    - [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
    - [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
    - [Docker](https://docs.docker.com/install/)
    - [Go](https://golang.org/doc/install)

1. Configure your enviroment so you can run tests.  Run:
    ```sh
    make test
    ```

1. We want to make sure we're starting fresh.  Tell Minikube to clear any previously running clusters.
    ```sh
    minikube delete
    ```

1. The [Minikube addon](https://github.com/kubernetes/minikube/blob/master/docs/addons.md) called [freshpod](https://github.com/GoogleCloudPlatform/freshpod) that will be very useful to us later.  You'll see.  It's gonna be cool.
    ```sh
    minikube addons enable freshpod
    ```

1. This part is really important.  You're going to set some environment variables which will intercept any images pulled by docker.  Run `minikube docker-env` to see what we're talking about.  You'll get an output that shows you what the script is doing.  Thankfully, it's not terribly complicated - it just sets some environment variables which will allow Minikube to man-in-the-middle the requests Kubernetes makes to pull images.  It will look something like this:
    ```
    export DOCKER_TLS_VERIFY="1"
    export DOCKER_HOST="tcp://192.168.99.128:2376"
    export DOCKER_CERT_PATH="/home/fluxrulez/.minikube/certs"
    export DOCKER_API_VERSION="1.35"
    # Run this command to configure your shell:
    # eval $(minikube docker-env)
    ```

    So, as the script suggests, run the following command:
    ```sh
    eval $(minikube docker-env)
    ```

    Now, be warned. These are local variables.  This means that if you run this `eval` in one terminal and then switch to another for later when we build the Flux project, you're gonna hit some issues.  For one, you'll know it isn't working because Kubernetes will tell you that it can't pull the image when you run `kubectl get pods`:

    ```
    NAME                        READY   STATUS         RESTARTS   AGE
    flux-7f6bd57699-shx9v       0/1     ErrImagePull   0          35s
    ```

## Prepare the Repository

1. Fork the [repo on GitHub](https://github.com/weaveworks/flux).

1. Clone `git@github.com:<YOUR-GITHUB-USERNAME>/flux.git` replacing `<YOUR-GITHUB-USERNAME>` with your GitHub username.

    In the same terminal you ran `eval $(minikube docker-env)`, run `GO111MODULE=on go mod download` followed by `make` from the root directory of the Flux repo.  You'll see docker's usual output as it builds the image layers.  Once it's done, you should see something like this in the middle of the output:
    ```
    Successfully built 606610e0f4ef
    Successfully tagged docker.io/fluxcd/flux:latest
    Successfully tagged docker.io/fluxcd/flux:master-a86167e4
    ```
    This confirms that a new docker image was tagged for your image.

1. Open up [`deploy/flux-deployment.yaml`](deploy/flux-deployment.yaml) and update the image at `spec.template.spec.containers[0].image` to be simply `docker.io/fluxcd/flux`.  While we're here, also change the `git-url` to point towards your fork.  It will look something like this in the yaml:
    ```yaml
    spec:
      template:
        spec:
          containers:
          - name: flux
            image: docker.io/fluxcd/flux
            imagePullPolicy: IfNotPresent
            args:
              - --git-url=git@github.com:<YOUR-GITHUB-USERNAME>/flux-getting-started
              - --git-branch=master
    ```

1. We're ready to apply your newly-customized deployment!  Since `kubectl` will apply all the Kubernetes manifests it finds (recursively) in a folder, we simply need to pass the directory to `kubectl apply`

    ```sh
    kubectl apply --filename ./deploy
    ````

    You should see an output similar to:

    ```
    serviceaccount/flux created
    clusterrole.rbac.authorization.k8s.io/flux created
    clusterrolebinding.rbac.authorization.k8s.io/flux created
    deployment.apps/flux created
    secret/flux-git-deploy created
    deployment.apps/memcached created
    service/memcached created
    secret/flux-git-deploy configured
    ```

    Congrats you just deployed your local Flux to your default namespace.  Check that everything is running:

    ```sh
    kubectl get pods --selector=name=flux
    ```

    You should get an output that looks like:
    ```sh
    NAME                   READY   STATUS    RESTARTS   AGE
    flux-6f7fd5bbc-hpq85   1/1     Running   0          38s
    ```

    If (instead) you see that Ready is showing `0/1` and/or the status is `ErrImagePull` double back on the instructions and make sure you did everything correctly and in order.

1. Pull the logs for your "fresh off of master" copy of Flux that you just deployed locally to Minikube:
    ```sh
    kubectl logs --selector=name=flux
    ```

    You should see an output that looks something like this:
    ```
    ts=2019-02-28T18:58:45.091531939Z caller=warming.go:268 component=warmer info="refreshing image" image=docker.io/fluxcd/flux tag_count=60 to_update=60 of_which_refresh=0 of_which_missing=60
    ts=2019-02-28T18:58:46.233723421Z caller=warming.go:364 component=warmer updated=docker.io/fluxcd/flux    successful=60 attempted=60
    ts=2019-02-28T18:58:46.234086642Z caller=images.go:17 component=sync-loop msg="polling images"
    ts=2019-02-28T18:58:46.234125646Z caller=images.go:27 component=sync-loop msg="no automated services"
    ts=2019-02-28T18:58:46.749598558Z caller=warming.go:268 component=warmer info="refreshing image" image=memcached    tag_count=66 to_update=66 of_which_refresh=0 of_which_missing=66
    ts=2019-02-28T18:58:51.017452675Z caller=warming.go:364 component=warmer updated=memcached successful=66 attempted=66
    ts=2019-02-28T18:58:51.020061586Z caller=images.go:17 component=sync-loop msg="polling images"
    ts=2019-02-28T18:58:51.020113243Z caller=images.go:27 component=sync-loop msg="no automated services"
    ```

## Make Some Changes

1. Now for the part you've been waiting for!  We're going to make a cosmetic change to our local copy of Flux.  Navigate to [git/operations.go](git/operations.go).  In it, you will find a private function to this package that goes by the name `execGitCmd`.  Paste the following as the (new) first line of the function:
    ```go
    fmt.Println("executing git command ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ")
    ```

1. Run `make` again.  Once this finishes you can check on your running pods with
    ```sh
    kubectl get pods --selector=name=flux
    ```

    Keep your eye on the `AGE` column.  It should be just a few seconds old if you check out the `AGE` column:

    ```sh
    NAME                   READY   STATUS    RESTARTS   AGE
    flux-6f7fd5bbc-6j9d5   1/1     Running   0          10s
    ```

    This pod was deployed even though we didn't run any `kubectl` commands or interact with Kubernetes directly because of the `freshpod` Minikube addon that we enabled earlier.  Freshpod saw that a new Docker image was tagged for `docker.io/fluxcd/flux:latest` and it went ahead and redeployed that pod for us.

    Consider that simply applying the `flux-deployment.yaml` file again wouldn't do anything since the actual image we're targeting (which is actually `docker.io/fluxcd/flux` with no `:latest` tag, but it's the same difference) hasn't changed.  The Kubernetes api server will get that JSON request from kubectl and go: "right... so nothing has changed in the file so I have nothing to do... IGNORE!".

    There is another way to do this, of course.  Remember that before when we ran `make` that we did _also_ get an image tagged with the `:<branch>-<commit hash>` syntax (in our specific example above it was `:master-a86167e4`).  We could, in theory, grab that tag every time we `make`, and then paste it into `spec.template.spec.containers[0].image` of our deployment.  That's tedious and error prone.  Instead, `freshpod` cuts this step out for us and accomplishes the same end goal.

2. Check the logs again (with `kubectl logs --selector=name=flux`) to find that your obnoxious chain of `Z`s is present.

## Congratulations!

You have now modified Flux and deployed that change locally.  From here on out, you simply need to run `make` after you save your changes and wait a few seconds for your new pod to be deployed to minikube.  Keep in mind, that (as in the situation where you run `make` without saving any changes) if the docker image you pointed to in the Kubernetes deployment for Flux is not Successfully tagged, `freshpod` won't have anything new to deploy.  Other than that, you should be good to go!
