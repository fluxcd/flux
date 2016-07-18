.DEFAULT: all
.PHONY: all clean

DOCKER?=docker

all: build/.fluxy.done

clean:
	go clean
	rm -rf ./build

build/.fluxy.done: docker/Dockerfile.fluxy build/fluxd ./cmd/fluxd/*.crt ./cmd/fluxd/entrypoint.bash
	mkdir -p ./build/docker
	cp $^ ./build/docker/
	${DOCKER} build -t weaveworks/fluxy -f build/docker/Dockerfile.fluxy ./build/docker
	touch $@

build/fluxd: cmd/fluxd/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ cmd/fluxd/main.go
