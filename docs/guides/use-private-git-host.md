# Using a private Git host

If you're using your own git host -- e.g., your own installation of
gitlab, or bitbucket server -- you will need to add its host key to
`~/.ssh/known_hosts` in the Flux daemon container.

First, run a check that you can clone the repo. The following assumes
that your git server's hostname (e.g., `githost`) is in `$GITHOST` and
the URL you'll use to access the repository (e.g.,
`user@githost:path/to/repo`) is in `$GITREPO`.

```sh
$ # Find the fluxd daemon pod:
$ kubectl get pods --all-namespaces -l name=flux
NAMESPACE   NAME                    READY     STATUS    RESTARTS   AGE
weave       flux-85cdc6cdfc-n2tgf   1/1       Running   0          1h

$ kubectl exec -n weave flux-85cdc6cdfc-n2tgf -ti -- \
    env GITHOST="$GITHOST" GITREPO="$GITREPO" PS1="container$ " /bin/sh

container$ git clone $GITREPO
Cloning into <repository name>...
No ECDSA host key is known for  <GITHOST> and you have requested strict checking.
Host key verification failed.
fatal: Could not read from remote repository

container$ # ^ that was expected. Now we'll try with a modified known_hosts
container$ ssh-keyscan $GITHOST >> ~/.ssh/known_hosts
container$ git clone $GITREPO
Cloning into '...'
...
```

If `git clone` doesn't succeed, you'll need to check that the SSH key
has been installed properly first, then come back. `ssh -vv $GITHOST`
from within the container may help debug it.

If it _did_ work, you will need to make it a more permanent
arrangement. Back in that shell, create a ConfigMap for the cluster. To
make sure the ConfigMap is created in the namespace of the Flux
deployment, the namespace is set explicitly:

```sh
container$ kubectl create configmap flux-ssh-config --from-file=$HOME/.ssh/known_hosts -n $(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)
configmap "flux-ssh-config" created
```

To use the ConfigMap every time the Flux daemon restarts, you'll need
to mount it into the container. The example deployment manifest
includes an example of doing this, commented out. Uncomment those two blocks:

```yaml
      - name: ssh-config
        configMap:
          name: flux-ssh-config
```

```yaml
        - name: ssh-config
          mountPath: /root/.ssh
```

It assumes you used `flux-ssh-config` as name of the ConfigMap and then reapply the
manifest.

Another alternative is to create the ConfigMap from a template. This could be
something like:

```yaml
apiVersion: v1
data:
  known_hosts: |
    # github
    192.30.253.112 ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==
    # github
    192.30.253.113 ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==
    # private gitlab
    gitlab.________ ssh-rsa AAAAB3N...
kind: ConfigMap
metadata:
  name: flux-ssh-config
  namespace: <OPTIONAL NAMESPACE (if not default)>
```

You will need to explicitly tell `fluxd` to use that service account by
uncommenting and possible adapting the line `# serviceAccountName:
flux` in the file `flux-deployment.yaml` before applying it.
