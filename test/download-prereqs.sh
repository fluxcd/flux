#!/bin/sh

set -e

if [ `uname -o` != 'GNU/Linux' -o `uname -m` != 'x86_64' ]; then
    echo "Sorry, this script only supports Linux amd64 for now." 1>&2
    exit 1
fi

arch=amd64
dldir="`dirname $0`/deps"
bindir="`dirname $0`/bin"

test -d "$dldir" || mkdir "$dldir"
test -d "$bindir" || mkdir "$bindir"

# helm
helm_base=https://storage.googleapis.com/kubernetes-helm
helm_version=v2.9.1
helm_relname=helm-$helm_version-linux-$arch.tar.gz
helm_dl=$dldir/$helm_relname
helm_bin=$bindir/helm

curl -s -L -o $helm_dl -z $helm_dl $helm_base/$helm_relname
test -f $helm_bin || test $helm_bin -ot $helm_dl || tar -z -x -f $helm_dl -C $bindir --strip-components 1 linux-$arch/helm

# kubectl
kubectl_base=https://storage.googleapis.com/kubernetes-release/release
kubectl_version=v1.10.3
kubectl_relname=kubectl_linux-$arch
kubectl_dl=$dldir/$kubectl_relname-$kubectl_version 
kubectl_bin=$bindir/kubectl

curl -s -L -o $kubectl_dl -z $kubectl_dl $kubectl_base/$kubectl_version/bin/linux/$arch/kubectl
chmod 755 $kubectl_dl
ln -f $kubectl_dl $kubectl_bin

# minikube
minikube_base=https://github.com/kubernetes/minikube/releases/download
minikube_version=v0.28.1
minikube_relname=minikube-linux-$arch
minikube_dl=$dldir/$minikube_relname-$minikube_version 
minikube_bin=$bindir/minikube

curl -s -L -o $minikube_dl -z $minikube_dl $minikube_base/$minikube_version/$minikube_relname
chmod 755 $minikube_dl
ln -f $minikube_dl $minikube_bin

# yq
yq_base=https://github.com/mikefarah/yq/releases/download/
yq_version=1.15.0
yq_relname=yq_linux_$arch
yq_dl=$dldir/$yq_relname-$yq_version 
yq_bin=$bindir/yq

curl -s -L -o $yq_dl -z $yq_dl $yq_base/$yq_version/$yq_relname
chmod 755 $yq_dl
ln -f $yq_dl $yq_bin
