# Flux Helm Operator

The Helm operator deals with Helm Chart releases. The operator watches for
changes of Custom Resources of kind FluxHelmRelease. It receives Kubernetes
Events and acts accordingly, installing, upgrading or deleting a Chart release.

## Setup and configuration

helm-operator requires setup and offers customization though a multitude of flags.
(TODO: change the flags to reflect reality)

|flag                    | default                       | purpose |
|------------------------|-------------------------------|---------|
|--kubernetes-kubectl          |                               | Optional, explicit path to kubectl tool.|
|--kubeconfig                  |                               | Path to a kubeconfig. Only required if out-of-cluster.|
|--master                      |                               | The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.|
|                              |                               | **Tiller options**|
|--tillerIP                    |                               | Tiller IP address. Only required if out-of-cluster.|
|--tillerPort                  |                               | Tiller port.|
|--tillerNamespace             |                               | Tiller namespace. If not provided, the default is kube-system.| |
|--tiller-tls-enable           |`false`                        | Enable TLS communication with Tiller. If provided, requires TLSKey and TLSCert to be provided as well. |
|--tiller-tls-verify           |`false`                        | Verify TLS certificate from Tiller. Will enable TLS communication when provided. |
|--tiller-tls-tls-key-path     |`/etc/fluxd/helm/tls.key`      | Path to private key file used to communicate with the Tiller server. |
|--tiller-tls-tls-cert-path    |`/etc/fluxd/helm/tls.crt`      | Path to certificate file used to communicate with the Tiller server. |
|--tiller-tls-tls-ca-cert-path |                               | Path to CA certificate file used to validate the Tiller server. Required if tiller-tls-verify is enabled. |
|                              |                               | **Git repo & key etc.**|
|--git-url                     |                               | URL of git repo with Helm Charts; e.g., `ssh://git@github.com/weaveworks/flux-example`|
|--git-branch                  | `master`                      | Branch of git repo to use for Kubernetes manifests|
|--git-charts-path             | `charts`                      | Path within git repo to locate Kubernetes Charts (relative path)|
|                              |                               | **repo chart changes** (none of these need overriding, usually) |
|--git-poll-interval           | `5 minutes`                   | period at which to poll git repo for new commits|
|--chartsSyncInterval          | 3*time.Minute                 | Interval at which to check for changed charts.|
|--chartsSyncTimeout           | 1*time.Minute                 | Timeout when checking for changed charts.|
|                              |                               | **k8s-secret backed ssh keyring configuration**|
|--k8s-secret-volume-mount-path | `/etc/fluxd/ssh`       | Mount location of the k8s secret storing the private SSH key|
|--k8s-secret-data-key         | `identity`                    | Data key holding the private SSH key within the k8s secret|
|--queueWorkerCount            |  2                            | Number of workers to process queue with Chart release jobs.|

## Installing Weave Flux helm-operator and Helm with TLS enabled

### Installing Helm / Tiller

Generate certificates for Tiller and Flux. This will provide a CA, servercerts for tiller and client certs for helm / weave flux.

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

### deploy weave flux helm-operator

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

#### Check if it worked

Use `kubectl logs` on the helm-operator and observe the helm client being created.

#### Debugging

##### Error creating helm client: failed to append certificates from file: /etc/fluxd/helm-ca/ca.crt

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
