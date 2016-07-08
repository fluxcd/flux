.DEFAULT: all
.PHONY: all clean

all: build/.fluxy.done

clean:
	go clean
	rm -rf ./build

build/.fluxy.done: docker/Dockerfile.fluxy build/fluxd
	mkdir -p ./build/docker
	cp $^ ./build/docker/
	docker build -t weaveworks/fluxy -f build/docker/Dockerfile.fluxy ./build/docker
	touch $@

build/fluxd:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ cmd/fluxd/main.go
