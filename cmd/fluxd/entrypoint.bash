#!/usr/bin/env bash

/home/flux/fluxd \
    --kubernetes \
    --kubernetes-kubectl=/home/flux/kubectl \
    --kubernetes-host="https://kubernetes" \
    --kubernetes-certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
    --kubernetes-bearer-token="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"
