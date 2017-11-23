A tool for examining the flux daemon's metadata cache.

    go build ./

### Accessing memcached

You will probably need to port forward to your memcache pod. Try

    kubectl get pod -n kube-system -o name -l name=memcached | \
    sed -e 's/pods\///' | \
    xargs -I{} kubectl -n kube-system port-forward {} 11211

If you're already using port `11211` (e.g., you already have memcached
running for another purpose, locally), forward another port and supply
the argument `--memcached` to `fluxcache`.

### Usage

    ./fluxcache [--memcached host:port] [--raw] [--key] item

where `item` is usually an image name (e.g.,
`quay.io/weaveworks/flux`) or an image ref
(`quay.io/weaveworks/flux:v1.1.0`).

The `--raw` flag makes it print out the data as its stored in the
cached (i.e., some JSON).

The `--key` argument means it will treat `item` as a memcache key
without intepretation, rather than treating it as an image name or
ref. Using `--key` means you will get raw output.
