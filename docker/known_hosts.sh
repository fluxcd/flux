#!/bin/sh

set -eu

known_hosts_file=${1}
known_hosts_file=${known_hosts_file:-/etc/ssh/ssh_known_hosts}

retries=10
count=0
ok=false
until ${ok}; do
    ssh-keyscan github.com gitlab.com bitbucket.org ssh.dev.azure.com vs-ssh.visualstudio.com >> ${known_hosts_file} && \
    sh /home/flux/verify_known_hosts.sh ${known_hosts_file} && ok=true || ok=false
    sleep 2
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
        echo "No more retries left"
        exit 1
    fi
done
