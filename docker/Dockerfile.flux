FROM alpine:3.11

WORKDIR /home/flux

RUN apk add --no-cache openssh-client ca-certificates tini 'git>=2.12.0' 'gnutls>=3.6.7' gnupg gawk socat
RUN apk add --no-cache -X http://dl-cdn.alpinelinux.org/alpine/edge/testing git-secret

# Add git hosts to known hosts file so we can use
# StrickHostKeyChecking with git+ssh
ADD ./known_hosts.sh /home/flux/known_hosts.sh
RUN sh /home/flux/known_hosts.sh /etc/ssh/ssh_known_hosts && \
    rm /home/flux/known_hosts.sh

# Add default SSH config, which points at the private key we'll mount
COPY ./ssh_config /etc/ssh/ssh_config

COPY ./kubectl /usr/local/bin/
COPY ./kustomize /usr/local/bin
COPY ./sops /usr/local/bin

# These are pretty static
LABEL maintainer="Flux CD <https://github.com/fluxcd/flux/issues>" \
      org.opencontainers.image.title="flux" \
      org.opencontainers.image.description="The GitOps operator for Kubernetes" \
      org.opencontainers.image.url="https://github.com/fluxcd/flux" \
      org.opencontainers.image.source="git@github.com:fluxcd/flux" \
      org.opencontainers.image.vendor="Flux CD" \
      org.label-schema.schema-version="1.0" \
      org.label-schema.name="flux" \
      org.label-schema.description="The GitOps operator for Kubernetes" \
      org.label-schema.url="https://github.com/fluxcd/flux" \
      org.label-schema.vcs-url="git@github.com:fluxcd/flux" \
      org.label-schema.vendor="Flux CD"

ENTRYPOINT [ "/sbin/tini", "--", "fluxd" ]

# Get the kubeyaml binary (files) and put them on the path
COPY --from=quay.io/squaremo/kubeyaml:0.7.0 /usr/lib/kubeyaml /usr/lib/kubeyaml/
ENV PATH=/bin:/usr/bin:/usr/local/bin:/usr/lib/kubeyaml

# Create minimal nsswitch.conf file to prioritize the usage of /etc/hosts over DNS queries.
# This resolves the conflict between:
# * fluxd using netgo for static compilation. netgo reads nsswitch.conf to mimic glibc,
#   defaulting to prioritize DNS queries over /etc/hosts if nsswitch.conf is missing:
#   https://github.com/golang/go/issues/22846
# * Alpine not including a nsswitch.conf file. Since Alpine doesn't use glibc
#   (it uses musl), maintainers argue that the need of nsswitch.conf is a Go bug:
#   https://github.com/gliderlabs/docker-alpine/issues/367#issuecomment-354316460
RUN [ ! -e /etc/nsswitch.conf ] && echo 'hosts: files dns' > /etc/nsswitch.conf
COPY ./kubeconfig /root/.kube/config
COPY ./fluxd /usr/local/bin/

ARG BUILD_DATE
ARG VCS_REF

# These will change for every build
LABEL org.opencontainers.image.revision="$VCS_REF" \
      org.opencontainers.image.created="$BUILD_DATE" \
      org.label-schema.vcs-ref="$VCS_REF" \
      org.label-schema.build-date="$BUILD_DATE"
