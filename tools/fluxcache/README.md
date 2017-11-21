A tool for examining the flux daemon's metadata cache.

    go build ./

Using: you will probably need to port forward to your memcache pod. Try

    kubectl get pod -n kube-system -o name -l name=memcached | \
    sed -e 's/pods\///' | \
    xargs -I{} kubectl -n kube-system port-forward {} 11211
