.DEFAULT: all
.PHONY: all clean

DOCKER?=docker
include docker/kubectl.version

LIBS:= . $(shell find platform -type d) $(shell find registry -type d)
LIB_SRC:=$(foreach lib,$(LIBS),$(wildcard $(lib)/*.go))

all: build/.fluxy.done

clean:
	go clean
	rm -rf ./build

build/.fluxy.done: docker/Dockerfile.fluxy build/fluxd ./cmd/fluxd/*.crt build/kubectl
	mkdir -p ./build/docker
	cp $^ ./build/docker/
	${DOCKER} build -t weaveworks/fluxy -f build/docker/Dockerfile.fluxy ./build/docker
	touch $@

build/fluxd: $(LIB_SRC) cmd/fluxd/*.go 
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ cmd/fluxd/main.go

build/kubectl: build/kubectl-$(KUBECTL_VERSION) docker/kubectl.version
	cp build/kubectl-$(KUBECTL_VERSION) $@
	chmod a+x $@

build/kubectl-$(KUBECTL_VERSION):
	curl -L -o $@ "https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl"
