# Flux Helm Operator

The Helm operator deals with Helm Chart releases. The operator watches for
changes of Custom Resources of kind HelmRelease. It receives Kubernetes
Events and acts accordingly, installing, upgrading or deleting a Chart release.

## Setup and configuration

helm-operator requires setup and offers customization though a multitude of flags.

|flag                          | default                       | purpose |
|------------------------------|-------------------------------|---------|
|--kubeconfig                  |                               | Path to a kubeconfig. Only required if out-of-cluster. |
|--master                      |                               | The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster. |
|--allow-namespace             |                               | If set, this limits the scope to a single namespace. if not specified, all namespaces will be watched. |
|                              |                               | **Tiller options** |
|--tiller-ip                   |                               | Tiller IP address. Only required if out-of-cluster. |
|--tiller-port                 |                               | Tiller port. |
|--tiller-namespace            |                               | Tiller namespace. If not provided, the default is kube-system. |
|--tiller-tls-enable           |`false`                        | Enable TLS communication with Tiller. If provided, requires TLSKey and TLSCert to be provided as well. |
|--tiller-tls-verify           |`false`                        | Verify TLS certificate from Tiller. Will enable TLS communication when provided. |
|--tiller-tls-key-path         |`/etc/fluxd/helm/tls.key`      | Path to private key file used to communicate with the Tiller server. |
|--tiller-tls-cert-path        |`/etc/fluxd/helm/tls.crt`      | Path to certificate file used to communicate with the Tiller server. |
|--tiller-tls-ca-cert-path     |                               | Path to CA certificate file used to validate the Tiller server. Required if tiller-tls-verify is enabled. |
|--tiller-tls-hostname         |                               | The server name used to verify the hostname on the returned certificates from the Tiller server. |
|                              |                               | **repo chart changes** (none of these need overriding, usually) |
|--charts-sync-interval        | `3m`                          | Interval at which to check for changed charts. |
|--git-timeout                 | `20s`                         | Duration after which git operations time out. |
|--log-release-diffs           | `false`                       | Log the diff when a chart release diverges. **Potentially insecure.** |
|--update-chart-deps           | `true`                        | Update chart dependencies before installing or upgrading a release. |

## Installing Weave Flux helm-operator and Helm with TLS enabled

### Installing Helm / Tiller

Generate certificates for Tiller and Flux. This will provide a CA, servercerts for Tiller and client certs for Helm / Weave Flux.

> **Note**: When creating the certificate for Tiller the Common Name should match the hostname you are connecting to from the Helm operator.

The following script can be used for that (requires [cfssl](https://github.com/cloudflare/cfssl)):

```bash
# TILLER_HOSTNAME=<service>.<namespace>
export TILLER_HOSTNAME=tiller-deploy.kube-system
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
helm --tls --tls-verify \
  --tls-ca-cert ./tls/ca.pem \
  --tls-cert ./tls/flux-helm-operator.pem \
  --tls-key ././tls/flux-helm-operator-key.pem \
  --tls-hostname tiller-deploy.kube-system \
  ls
```

### deploy weave flux helm-operator

First create a new Kubernetes TLS secret for the client certs;

```bash
kubectl create secret tls helm-client --cert=tls/flux-helm-operator.pem --key=./tls/flux-helm-operator-key.pem
```

> note: this has to be in the same namespace as the flux-helm-operator is deployed in.

Deploy Flux with Helm;

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
> note:
> - include --tls flags for `helm` as in the `helm ls` example, if talking to a tiller with TLS
> - optionally specify target --namespace

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

## Installing Weave Flux helm-operator for Weave Cloud

In order to use the Helm operator with Weave Cloud you have to apply the `HelmRelease` CRD definition and the operator 
deployment in the `weave` namespace:

```bash
export REPO=https://raw.githubusercontent.com/weaveworks/flux/master

kubectl apply -f ${REPO}/deploy-helm/flux-helm-release-crd.yaml
kubectl apply -f ${REPO}/deploy-helm/weave-cloud-helm-operator-deployment.yaml
```

Check the operator logs with:

```bash
kubectl -n weave logs deployment/flux-helm-operator -f
```

**Note:** that the above instructions are assuming that Tiller is deployed in the `kube-system` namespace without TLS.
