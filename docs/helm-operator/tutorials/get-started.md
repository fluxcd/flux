# Get started with the Helm operator

## Installing Helm / Tiller

Generate certificates for Tiller and Flux. This will provide a CA, servercerts for Tiller and client certs for Helm / Flux.

> **Note:** When creating the certificate for Tiller the Common Name should match the hostname you are connecting to from the Helm operator.

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

# Deploy helm with mutual TLS enabled.
# --history-max limits the maximum number of revisions Tiller stores;
# leaving it to the default (0) may result in request timeouts after N
# releases, due to the excessive amount of ConfigMaps Tiller will
# attempt to retrieve.
helm init --upgrade --service-account tiller --history-max 10 \
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

## Deploy the Helm Operator

First create a new Kubernetes TLS secret for the client certs;

```bash
kubectl create secret tls helm-client --cert=tls/flux-helm-operator.pem --key=./tls/flux-helm-operator-key.pem
```

> **Note:** this has to be in the same namespace as the flux-helm-operator is deployed in.

Deploy Flux with Helm;

```bash
helm repo add fluxcd https://charts.fluxcd.io

helm upgrade --install \
    --set helmOperator.create=true \
    --set git.url=$YOUR_GIT_REPO \
    --set helmOperator.tls.enable=true \
    --set helmOperator.tls.verify=true \
    --set helmOperator.tls.secretName=helm-client \
    --set helmOperator.tls.caContent="$(cat ./tls/ca.pem)" \
    flux \
    fluxcd/flux
```

> **Note:**
>
> - include --tls flags for `helm` as in the `helm ls` example, if talking to a tiller with TLS
> - optionally specify target --namespace

## Check if it worked

Use `kubectl logs` on the Helm Operator and observe the helm client being created.
