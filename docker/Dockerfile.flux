FROM alpine:3.5
WORKDIR /home/flux
ENTRYPOINT [ "/sbin/tini", "--", "fluxd" ]
RUN apk add --no-cache openssh ca-certificates tini 'git>=2.3.0'

# Add git hosts to known hosts file so when git ssh's using the deploy
# key we don't get an unknown host warning.
RUN mkdir ~/.ssh && touch ~/.ssh/known_hosts && \
    ssh-keyscan github.com gitlab.com bitbucket.org >> ~/.ssh/known_hosts && \
    chmod 600 ~/.ssh/known_hosts

COPY ./kubectl /usr/local/bin/
COPY ./fluxd /usr/local/bin/
