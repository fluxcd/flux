.DEFAULT: all
.PHONY: all release-bins clean realclean test integration-test

DOCKER?=docker
TEST_FLAGS?=

include docker/kubectl.version

# NB because this outputs absolute file names, you have to be careful
# if you're testing out the Makefile with `-W` (pretend a file is
# new); use the full path to the pretend-new file, e.g.,
#  `make -W $PWD/registry/registry.go`
godeps=$(shell go list -f '{{join .Deps "\n"}}' $1 | grep -v /vendor/ | xargs go list -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}')

FLUXD_DEPS:=$(call godeps,./cmd/fluxd)
FLUXCTL_DEPS:=$(call godeps,./cmd/fluxctl)
HELM_OPERATOR_DEPS:=$(call godeps,./cmd/helm-operator)

IMAGE_TAG:=$(shell ./docker/image-tag)

all: $(GOPATH)/bin/fluxctl $(GOPATH)/bin/fluxd $(GOPATH)/bin/helm-operator build/.flux.done build/.helm-operator.done

release-bins:
	for arch in amd64; do \
		for os in linux darwin; do \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o "build/fluxctl_"$$os"_$$arch" $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxctl/; \
		done; \
	done

clean:
	go clean
	rm -rf ./build

realclean: clean
	rm -rf ./cache

test:
	go test ${TEST_FLAGS} $(shell go list ./... | grep -v "^github.com/weaveworks/flux/vendor" | sort -u)

build/.%.done: docker/Dockerfile.%
	mkdir -p ./build/docker/$*
	cp $^ ./build/docker/$*/
	${DOCKER} build -t quay.io/weaveworks/$* -t quay.io/weaveworks/$*:$(IMAGE_TAG) -f build/docker/$*/Dockerfile.$* ./build/docker/$*
	touch $@

build/.flux.done: build/fluxd build/kubectl docker/ssh_config
build/.helm-operator.done: build/helm-operator build/kubectl docker/ssh_config

build/fluxd: $(FLUXD_DEPS)
build/fluxd: cmd/fluxd/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxd

build/helm-operator: $(HELM_OPERATOR_DEPS)
build/helm-operator: cmd/helm-operator/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/helm-operator

build/kubectl: cache/kubectl-$(KUBECTL_VERSION) docker/kubectl.version
	cp cache/kubectl-$(KUBECTL_VERSION) $@
	strip $@
	chmod a+x $@

cache/kubectl-$(KUBECTL_VERSION):
	mkdir -p cache
	curl -L -o $@ "https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl"

$(GOPATH)/bin/fluxctl: $(FLUXCTL_DEPS)
$(GOPATH)/bin/fluxctl: ./cmd/fluxctl/*.go
	go install ./cmd/fluxctl

$(GOPATH)/bin/fluxd: $(FLUXD_DEPS)
$(GOPATH)/bin/fluxd: cmd/fluxd/*.go
	go install ./cmd/fluxd

$(GOPATH)/bin/helm-operator: $(HELM_OPERATOR_DEPS)
$(GOPATH)/bin/help-operator: cmd/helm-operator/*.go
	go install ./cmd/helm-operator

integration-test: all
	test/bin/test-flux
