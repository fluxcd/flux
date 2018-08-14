FROM alpine:3.6

WORKDIR /home/flux

RUN apk add --no-cache openssh ca-certificates tini 'git>=2.3.0'

# Add git hosts to known hosts file so we can use
# StrickHostKeyChecking with git+ssh
RUN ssh-keyscan github.com gitlab.com bitbucket.org >> /etc/ssh/ssh_known_hosts

# Verify newly added known_hosts (man-in-middle mitigation)
ADD ./verify_known_hosts.sh /home/flux/verify_known_hosts.sh
RUN sh /home/flux/verify_known_hosts.sh /etc/ssh/ssh_known_hosts && rm /home/flux/verify_known_hosts.sh

# Add default SSH config, which points at the private key we'll mount
COPY ./ssh_config /etc/ssh/ssh_config

COPY ./kubectl /usr/local/bin/

# These are pretty static
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="flux" \
      org.opencontainers.image.description="The Flux daemon, for synchronising your cluster with a git repo, and deploying new images" \
      org.opencontainers.image.url="https://github.com/weaveworks/flux" \
      org.opencontainers.image.source="git@github.com:weaveworks/flux" \
      org.opencontainers.image.vendor="Weaveworks" \
      org.label-schema.schema-version="1.0" \
      org.label-schema.name="flux" \
      org.label-schema.description="The Flux daemon, for synchronising your cluster with a git repo, and deploying new images" \
      org.label-schema.url="https://github.com/weaveworks/flux" \
      org.label-schema.vcs-url="git@github.com:weaveworks/flux" \
      org.label-schema.vendor="Weaveworks"

ENTRYPOINT [ "/sbin/tini", "--", "fluxd" ]

# Get the kubeyaml binary (files) and put them on the path
COPY --from=quay.io/squaremo/kubeyaml:0.4.2 /usr/lib/kubeyaml /usr/lib/kubeyaml/
ENV PATH=/bin:/usr/bin:/usr/local/bin:/usr/lib/kubeyaml

COPY ./kubeconfig /root/.kube/config
COPY ./fluxd /usr/local/bin/

ARG BUILD_DATE
ARG VCS_REF

# These will change for every build
LABEL org.opencontainers.image.revision="$VCS_REF" \
      org.opencontainers.image.created="$BUILD_DATE" \
      org.label-schema.vcs-ref="$VCS_REF" \
      org.label-schema.build-date="$BUILD_DATE"
