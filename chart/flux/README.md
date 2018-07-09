# Flux

Flux is a tool that automatically ensures that the state of a cluster matches the config in git. 
It uses an operator in the cluster to trigger deployments inside Kubernetes, which means you don't need a separate CD tool. 
It monitors all relevant image repositories, detects new images, triggers deployments and updates the desired running
configuration based on that (and a configurable policy).

## Introduction

This chart bootstraps a [Flux](https://github.com/weaveworks/flux) deployment on 
a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.9+

## Installing the Chart

Add the weaveworks repo:

```
helm repo add weaveworks https://weaveworks.github.io/flux
```

To install the chart with the release name `flux`:

```console
$ helm install --name flux \
--set git.url=ssh://git@github.com/weaveworks/flux-example \
--namespace flux \
weaveworks/flux
```

To connect Flux to a Weave Cloud instance:

```console
helm install --name flux \
--set token=YOUR_WEAVE_CLOUD_SERVICE_TOKEN \
--namespace flux \
weaveworks/flux
```

To install Flux with the Helm operator (alpha version):

```console
$ helm install --name flux \
--set git.url=ssh://git@github.com/weaveworks/flux-helm-test \
--set helmOperator.create=true \
--namespace flux \
weaveworks/flux
```

The [configuration](#configuration) section lists the parameters that can be configured during installation.

### Setup Git deploy 

At startup Flux generates a SSH key and logs the public key. 
Find the SSH public key with:

```bash
kubectl -n flux logs deployment/flux | grep identity.pub | cut -d '"' -f2
```

In order to sync your cluster state with GitHub you need to copy the public key and 
create a deploy key with write access on your GitHub repository.
Go to _Settings > Deploy keys_ click on _Add deploy key_, check 
_Allow write access_, paste the Flux public key and click _Add key_.

## Uninstalling the Chart

To uninstall/delete the `flux` deployment:

```console
$ helm delete --purge flux
```

The command removes all the Kubernetes components associated with the chart and deletes the release. 
You should also remove the deploy key from your GitHub repository.

## Configuration

The following tables lists the configurable parameters of the Weave Flux chart and their default values.

| Parameter                       | Description                                | Default                                                    |
| ------------------------------- | ------------------------------------------ | ---------------------------------------------------------- |
| `image.repository` | Image repository | `quay.io/weaveworks/flux` 
| `image.tag` | Image tag | `1.3.1` 
| `image.pullPolicy` | Image pull policy | `IfNotPresent` 
| `resources` | CPU/memory resource requests/limits | None 
| `rbac.create` | If `true`, create and use RBAC resources | `true`
| `serviceAccount.create` | If `true`, create a new service account | `true`
| `serviceAccount.name` | Service account to be used | `flux`
| `service.type` | Service type to be used (exposing the Flux API outside of the cluster is not advised) | `ClusterIP`
| `service.port` | Service port to be used | `3030`
| `git.url` | URL of git repo with Kubernetes manifests | None
| `git.branch` | Branch of git repo to use for Kubernetes manifests | `master`
| `git.path` | Path within git repo to locate Kubernetes manifests (relative path) | None
| `git.user` | Username to use as git committer | `Weave Flux`
| `git.email` | Email to use as git committer | `support@weave.works`
| `git.chartsPath` | Path within git repo to locate Helm charts (relative path) | `charts`
| `git.pollInterval` | Period at which to poll git repo for new commits | `30s`
| `ssh.known_hosts`  | The contents of an SSH `known_hosts` file, if you need to supply host key(s) |
| `helmOperator.create` | If `true`, install the Helm operator | `false`
| `helmOperator.repository` | Helm operator image repository | `quay.io/weaveworks/helm-operator` 
| `helmOperator.tag` | Helm operator image tag | `0.1.0-alpha` 
| `helmOperator.pullPolicy` | Helm operator image pull policy | `IfNotPresent` 
| `helmOperator.tillerNamespace` | Namespace in which the Tiller server can be found | `kube-system`
| `helmOperator.tls.enable` | Enable TLS for communicating with Tiller | `false`
| `helmOperator.tls.verify` | Verify the Tiller certificate, also enables TLS when set to true | `false`
| `helmOperator.tls.secretName` | Name of the secret containing the TLS client certificates for communicating with Tiller | `helm-client-certs`
| `helmOperator.tls.keyFile` | Name of the key file within the k8s secret | `tls.key`
| `helmOperator.tls.certFile` | Name of the certificate file within the k8s secret | `tls.crt`
| `helmOperator.tls.caContent` | Certificate Authority content used to validate the Tiller server certificate | None
| `token` | Weave Cloud service token | None 

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example:

```console
$ helm upgrade --install --wait flux \
--set git.url=ssh://git@github.com/stefanprodan/podinfo \
--set git.path=deploy/auto-scaling \
--namespace flux \
weaveworks/flux
```

## Upgrade

Update Weave Flux version with:

```console
helm upgrade --reuse-values flux \
--set image.tag=1.3.2 \
weaveworks/flux
```

## Installing Weave Flux helm-operator and Helm with TLS enabled

### Installing Helm / Tiller

Generate certificates for Tiller and Flux. This will provide a CA, server certs for tiller and client certs for helm / weave flux. 

The following script can be used for that (requires [cfssl](https://github.com/cloudflare/cfssl)):

```bash
export TILLER_HOSTNAME=tiller-server
export TILLER_SERVER=server
export USER_NAME=flux-helm-operator

mkdir tls
cd ./tls

# Prep the configuration
echo '{"CN":"CA","key":{"algo":"rsa","size":4096}}' | cfssl gencert -initca - | cfssljson -bare ca -
echo '{"signing":{"default":{"expiry":"43800h","usages":["signing","key encipherment","server auth","client auth"]}}}' > ca-config.json

# Create the tiller certificate
echo '{"CN":"'$TILLER_SERVER'","hosts":[""],"key":{"algo":"rsa","size":4096}}' | cfssl gencert \
  -config=ca-config.json -ca=ca.pem \
  -ca-key=ca-key.pem \
  -hostname="$TILLER_HOSTNAME" - | cfssljson -bare $TILLER_SERVER

# Create a client certificate
echo '{"CN":"'$USER_NAME'","hosts":[""],"key":{"algo":"rsa","size":4096}}' | cfssl gencert \
  -config=ca-config.json -ca=ca.pem -ca-key=ca-key.pem \
  -hostname="$TILLER_HOSTNAME" - | cfssljson -bare $USER_NAME
```

Alternatively, you can follow the [Helm documentation for configuring TLS](https://docs.helm.sh/using_helm/#using-ssl-between-helm-and-tiller).


Next deploy Helm with TLS and RBAC enabled; 

Create a file called `helm-rbac.yaml`. This contains all the RBAC configuration for Tiller:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tiller
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: tiller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: tiller
    namespace: kube-system

---
# Helm client serviceaccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: helm
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: Role
metadata:
  name: tiller-user
  namespace: kube-system
rules:
- apiGroups:
  - ""
  resources:
  - pods/portforward
  verbs:
  - create
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - list
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: RoleBinding
metadata:
  name: tiller-user-binding
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: tiller-user
subjects:
- kind: ServiceAccount
  name: helm
  namespace: kube-system
```

Deploy Tiller:

```bash
kubectl apply -f helm-rbac.yaml

# Deploy helm with mutual TLS enabled
helm init --upgrade --service-account tiller \
    --override 'spec.template.spec.containers[0].command'='{/tiller,--storage=secret}' \
    --tiller-tls \
    --tiller-tls-cert ./tls/server.pem \
    --tiller-tls-key ./tls/server-key.pem \
    --tiller-tls-verify \
    --tls-ca-cert ./tls/ca.pem
```

To check if Tiller installed succesfully with TLS enabled, try `helm ls`. This should give an error:

```bash
# Should give an error
$ helm ls
Error: transport is closing
```

When providing the certificates, it should work correctly:

```bash
helm --tls \
  --tls-ca-cert ./tls/ca.pem \
  --tls-cert ./tls/flux-helm-operator.pem \
  --tls-key ././tls/flux-helm-operator-key.pem \
  ls
```

## deploy weave flux helm-operator

First create a new Kubernetes TLS secret for the client certs;

```bash
kubectl create secret tls helm-client --cert=tls/flux-helm-operator.pem --key=./tls/flux-helm-operator-key.pem
```

> note; this has to be in the same namespace as the helm-operator is deployed in.

Deploy flux with Helm;

```bash
helm repo add weaveworks https://weaveworks.github.io/flux

helm upgrade --install \
    --set helmOperator.create=true \
    --set git.url=$YOUR_GIT_REPO \
    --set helmOperator.tls.enable=true \
    --set helmOperator.tls.verify=true \    
    --set helmOperator.tls.secretName=helm-client \
    --set helmOperator.tls.caContent="$(cat ./tls/ca.pem)" \
    flux \
    weaveworks/flux
```

### Check if it worked

Use `kubectl logs` on the helm-operator and observe the helm client being created.

### Debugging

#### Error creating helm client: failed to append certificates from file: /etc/fluxd/helm-ca/ca.crt

Your CA certificate content is not set correctly, check if your configMap contains the correct values. Example:

```bash
$ kubectl get configmaps flux-helm-tls-ca-config -o yaml
apiVersion: v1
data:
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    ....
    -----END CERTIFICATE-----
kind: ConfigMap
metadata:
  creationTimestamp: 2018-07-04T15:27:25Z
  name: flux-helm-tls-ca-config
  namespace: helm-system
  resourceVersion: "1267257"
  selfLink: /api/v1/namespaces/helm-system/configmaps/flux-helm-tls-ca-config
  uid: c106f866-7f9e-11e8-904a-025000000001
```
