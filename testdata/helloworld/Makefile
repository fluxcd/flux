.PHONY: all
all: .helloworld

REV=a000001

.helloworld: helloworld-linux-amd64 Dockerfile
	docker build -t quay.io/weaveworks/helloworld:master-${REV} .
	touch $@

helloworld-linux-amd64: helloworld.go
	env GOOS=linux GOARCH=amd64 go build -ldflags="-X main.rev=${REV}" -o $@ $*

.PHONY: clean
clean:
	rm -f .helloworld helloworld-linux-amd64

