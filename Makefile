.DEFAULT: all
.PHONY: all release-bins clean realclean test integration-test

SUDO := $(shell docker info > /dev/null 2> /dev/null || echo "sudo")
TEST_FLAGS?=

include docker/kubectl.version
include docker/helm.version

HELM_TARGZ=./cache/helm-$(HELM_VERSION).tar.gz
KUBECTL_TARGZ=./cache/kubectl-$(KUBECTL_VERSION).tar.gz

# NB because this outputs absolute file names, you have to be careful
# if you're testing out the Makefile with `-W` (pretend a file is
# new); use the full path to the pretend-new file, e.g.,
#  `make -W $PWD/registry/registry.go`
godeps=$(shell go list -f '{{join .Deps "\n"}}' $1 | grep -v /vendor/ | xargs go list -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}')

FLUXD_DEPS:=$(call godeps,./cmd/fluxd)
FLUXCTL_DEPS:=$(call godeps,./cmd/fluxctl)
HELM_OPERATOR_DEPS:=$(call godeps,./cmd/helm-operator)

IMAGE_TAG:=$(shell ./docker/image-tag)
VCS_REF:=$(shell git rev-parse HEAD)
BUILD_DATE:=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

all: $(GOPATH)/bin/fluxctl $(GOPATH)/bin/fluxd $(GOPATH)/bin/helm-operator build/.flux.done build/.helm-operator.done

release-bins:
	for arch in amd64; do \
		for os in linux darwin windows; do \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o "build/fluxctl_"$$os"_$$arch" $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxctl/; \
		done; \
	done

clean:
	go clean
	rm -rf ./build

realclean: clean
	rm -rf ./cache

test:
	PATH=${PATH}:${PWD}/bin go test ${TEST_FLAGS} $(shell go list ./... | grep -v "^github.com/weaveworks/flux/vendor" | sort -u)

build/.%.done: docker/Dockerfile.%
	mkdir -p ./build/docker/$*
	cp $^ ./build/docker/$*/
	$(SUDO) docker build -t quay.io/weaveworks/$* -t quay.io/weaveworks/$*:$(IMAGE_TAG) \
		--build-arg VCS_REF="$(VCS_REF)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		-f build/docker/$*/Dockerfile.$* ./build/docker/$*
	touch $@

build/.flux.done: build/fluxd build/kubectl docker/ssh_config docker/kubeconfig docker/verify_known_hosts.sh
build/.helm-operator.done: build/helm-operator build/kubectl build/helm docker/ssh_config docker/verify_known_hosts.sh docker/helm-repositories.yaml

build/fluxd: $(FLUXD_DEPS)
build/fluxd: cmd/fluxd/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxd

build/helm-operator: $(HELM_OPERATOR_DEPS)
build/helm-operator: cmd/helm-operator/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/helm-operator

build/kubectl: cache/kubectl-$(KUBECTL_VERSION)
	cp cache/kubectl-$(KUBECTL_VERSION) $@
	strip $@
	chmod a+x $@

build/helm: cache/helm-$(HELM_VERSION)
	cp cache/helm-$(HELM_VERSION) $@
	strip $@
	chmod a+x $@

cache/kubectl-$(KUBECTL_VERSION): docker/kubectl.version
	mkdir -p cache
	curl -L -o $(KUBECTL_TARGZ) "https://dl.k8s.io/$(KUBECTL_VERSION)/kubernetes-client-linux-amd64.tar.gz"
	echo "$(KUBECTL_CHECKSUM) $(KUBECTL_TARGZ)" > "$(KUBECTL_TARGZ).checksum"
	sha256sum -c $(KUBECTL_TARGZ).checksum
	tar -C ./cache -xzf $(KUBECTL_TARGZ) kubernetes/client/bin/kubectl
	cp ./cache/kubernetes/client/bin/kubectl $@

cache/helm-$(HELM_VERSION): docker/helm.version
	mkdir -p cache
	curl -L -o $(HELM_TARGZ) "https://storage.googleapis.com/kubernetes-helm/helm-v$(HELM_VERSION)-linux-amd64.tar.gz"
	echo "$(HELM_CHECKSUM) $(HELM_TARGZ)" > "$(HELM_TARGZ).checksum"
	sha256sum -c "$(HELM_TARGZ).checksum"
	tar -C ./cache -xzf $(HELM_TARGZ) linux-amd64/helm
	cp ./cache/linux-amd64/helm $@

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
