.DEFAULT: all
.PHONY: all release-bins clean realclean test integration-test check-generated

SUDO := $(shell docker info > /dev/null 2> /dev/null || echo "sudo")

TEST_FLAGS?=

include docker/kubectl.version
include docker/helm.version

# NB default target architecture is amd64. If you would like to try the
# other one -- pass an ARCH variable, e.g.,
#  `make ARCH=arm64`
ifeq ($(ARCH),)
	ARCH=amd64
endif
CURRENT_OS_ARCH=$(shell echo `go env GOOS`-`go env GOARCH`)

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
	function do_build() { \
		os=$$1 \
		arch=$$2 \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o "build/fluxctl_"$$os"_$$arch" $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxctl/; \
	};\
	for arch in amd64; do \
		for os in linux darwin windows; do \
			do_build "$$os" "$$arch"; \
		done; \
	done; \
	for arch in arm arm64; do \
		for os in linux; do \
			do_build "$$os" "$$arch"; \
		done; \
	done;

clean:
	go clean
	rm -rf ./build
	rm -f test/bin/kubectl test/bin/helm

realclean: clean
	rm -rf ./cache

test: test/bin/helm test/bin/kubectl
	PATH="${PWD}/bin:${PWD}/test/bin:${PATH}" go test ${TEST_FLAGS} $(shell go list ./... | grep -v "^github.com/weaveworks/flux/vendor" | sort -u)

build/.%.done: docker/Dockerfile.%
	mkdir -p ./build/docker/$*
	cp $^ ./build/docker/$*/
	$(SUDO) docker build -t docker.io/weaveworks/$* -t docker.io/weaveworks/$*:$(IMAGE_TAG) \
		--build-arg VCS_REF="$(VCS_REF)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		-f build/docker/$*/Dockerfile.$* ./build/docker/$*
	touch $@

build/.flux.done: build/fluxd build/kubectl docker/ssh_config docker/kubeconfig docker/verify_known_hosts.sh
build/.helm-operator.done: build/helm-operator build/kubectl build/helm docker/ssh_config docker/verify_known_hosts.sh docker/helm-repositories.yaml

build/fluxd: $(FLUXD_DEPS)
build/fluxd: cmd/fluxd/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -o $@ $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxd

build/helm-operator: $(HELM_OPERATOR_DEPS)
build/helm-operator: cmd/helm-operator/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -o $@ $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/helm-operator

build/kubectl: cache/linux-$(ARCH)/kubectl-$(KUBECTL_VERSION)
test/bin/kubectl: cache/$(CURRENT_OS_ARCH)/kubectl-$(KUBECTL_VERSION)
build/helm: cache/linux-$(ARCH)/helm-$(HELM_VERSION)
test/bin/helm: cache/$(CURRENT_OS_ARCH)/helm-$(HELM_VERSION)
build/kubectl test/bin/kubectl build/helm test/bin/helm:
	mkdir -p build
	cp $< $@
	if [ `basename $@` = "build" -a $(CURRENT_OS_ARCH) = "linux-$(ARCH)" ]; then strip $@; fi
	chmod a+x $@

cache/%/kubectl-$(KUBECTL_VERSION): docker/kubectl.version
	mkdir -p cache/$*
	curl -L -o cache/$*/kubectl-$(KUBECTL_VERSION).tar.gz "https://dl.k8s.io/$(KUBECTL_VERSION)/kubernetes-client-$*.tar.gz"
	[ $* != "linux-$(ARCH)" ] || echo "$(KUBECTL_CHECKSUM_$(ARCH))  cache/$*/kubectl-$(KUBECTL_VERSION).tar.gz" | shasum -a 256 -c
	tar -m --strip-components 3 -C ./cache/$* -xzf cache/$*/kubectl-$(KUBECTL_VERSION).tar.gz kubernetes/client/bin/kubectl
	mv ./cache/$*/kubectl $@

cache/%/helm-$(HELM_VERSION): docker/helm.version
	mkdir -p cache/$*
	curl -L -o cache/$*/helm-$(HELM_VERSION).tar.gz "https://storage.googleapis.com/kubernetes-helm/helm-v$(HELM_VERSION)-$*.tar.gz"
	[ $* != "linux-$(ARCH)" ] || echo "$(HELM_CHECKSUM_$(ARCH))  cache/$*/helm-$(HELM_VERSION).tar.gz" | shasum -a 256 -c
	tar -m -C ./cache -xzf cache/$*/helm-$(HELM_VERSION).tar.gz $*/helm
	mv cache/$*/helm $@

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

check-generated:
	./bin/helm/update_codegen.sh
	git diff --exit-code -- integrations/apis intergrations/client

