FROM alpine:3.6
WORKDIR /home/flux
ENTRYPOINT [ "/sbin/tini", "--", "fluxsvc" ]
RUN apk add --no-cache ca-certificates tini
ADD ./migrations.tar /home/flux/
COPY ./fluxsvc /usr/local/bin/
