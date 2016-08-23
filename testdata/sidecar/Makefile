.PHONY: all
all: .sidecar

REV=a000002

.sidecar: sidecar-linux-amd64 Dockerfile
	docker build -t quay.io/weaveworks/sidecar:master-${REV} .
	touch $@

sidecar-linux-amd64: sidecar.go Makefile
	env GOOS=linux GOARCH=amd64 go build -ldflags="-X main.rev=${REV}" -o $@ $*

.PHONY: clean
clean:
	rm -f .sidecar sidecar-linux-amd64

