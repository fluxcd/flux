#!/bin/sh

set -eu

hosts=${@}
n=0
max=5
delay=10

while true
do
    ssh-keyscan ${hosts} > /etc/ssh/ssh_known_hosts \
        && sh ./verify_known_hosts.sh /etc/ssh/ssh_known_hosts \
        && break

    if [ $n -lt $max ]; then
        n=$((n+1))
        echo "Failed to gather and validate all SSH fingerprints. Attempt $n/$max."
        sleep $delay
    else
        echo "Failed to gather and validate all SSH fingerprints after $n attempts." >&2
        exit 1
    fi
done
