.DEFAULT: all
.PHONY: all release-bins clean realclean test integration-test generate-deploy check-generated

SUDO := $(shell docker info > /dev/null 2> /dev/null || echo "sudo")

TEST_FLAGS?=

include docker/kubectl.version
include docker/kustomize.version
include docker/helm.version

# NB default target architecture is amd64. If you would like to try the
# other one -- pass an ARCH variable, e.g.,
#  `make ARCH=arm64`
ifeq ($(ARCH),)
	ARCH=amd64
endif
CURRENT_OS_ARCH=$(shell echo `go env GOOS`-`go env GOARCH`)
GOBIN?=$(shell echo `go env GOPATH`/bin)

godeps=$(shell go list -deps -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}' $(1) | sed "s%${PWD}/%%g")

FLUXD_DEPS:=$(call godeps,./cmd/fluxd/...)
FLUXCTL_DEPS:=$(call godeps,./cmd/fluxctl/...)

IMAGE_TAG:=$(shell ./docker/image-tag)
VCS_REF:=$(shell git rev-parse HEAD)
BUILD_DATE:=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

DOCS_PORT:=8000

all: $(GOBIN)/fluxctl $(GOBIN)/fluxd build/.flux.done

release-bins:
	for arch in amd64; do \
		for os in linux darwin windows; do \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o "build/fluxctl_"$$os"_$$arch" $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxctl/; \
		done; \
	done;
	for arch in arm arm64; do \
		for os in linux; do \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o "build/fluxctl_"$$os"_$$arch" $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxctl/; \
		done; \
	done;

clean:
	go clean
	rm -rf ./build
	rm -f test/bin/kubectl test/bin/helm test/bin/kind test/bin/kustomize

realclean: clean
	rm -rf ./cache

test: test/bin/helm test/bin/kubectl test/bin/kustomize
	PATH="${PWD}/bin:${PWD}/test/bin:${PATH}" go test ${TEST_FLAGS} $(shell go list ./... | grep -v "^github.com/weaveworks/flux/vendor" | sort -u)

e2e: test/bin/helm test/bin/kubectl build/.flux.done
	PATH="${PWD}/test/bin:${PATH}" CURRENT_OS_ARCH=$(CURRENT_OS_ARCH) test/e2e/run.sh

build/.%.done: docker/Dockerfile.%
	mkdir -p ./build/docker/$*
	cp $^ ./build/docker/$*/
	$(SUDO) docker build -t docker.io/fluxcd/$* -t docker.io/fluxcd/$*:$(IMAGE_TAG) \
		--build-arg VCS_REF="$(VCS_REF)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		-f build/docker/$*/Dockerfile.$* ./build/docker/$*
	touch $@

build/.flux.done: build/fluxd build/kubectl build/kustomize docker/ssh_config docker/kubeconfig docker/known_hosts.sh

build/fluxd: $(FLUXD_DEPS)
build/fluxd: cmd/fluxd/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -o $@ $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxd

build/kubectl: cache/linux-$(ARCH)/kubectl-$(KUBECTL_VERSION)
test/bin/kubectl: cache/$(CURRENT_OS_ARCH)/kubectl-$(KUBECTL_VERSION)
build/helm: cache/linux-$(ARCH)/helm-$(HELM_VERSION)
test/bin/helm: cache/$(CURRENT_OS_ARCH)/helm-$(HELM_VERSION)
build/kustomize: cache/linux-amd64/kustomize-$(KUSTOMIZE_VERSION)
test/bin/kustomize: cache/$(CURRENT_OS_ARCH)/kustomize-$(KUSTOMIZE_VERSION)

build/kubectl test/bin/kubectl build/kustomize test/bin/kustomize build/helm test/bin/helm:
	mkdir -p build
	cp $< $@
	if [ `basename $@` = "build" -a $(CURRENT_OS_ARCH) = "linux-$(ARCH)" ]; then strip $@; fi
	chmod a+x $@

cache/%/kubectl-$(KUBECTL_VERSION): docker/kubectl.version
	mkdir -p cache/$*
	curl --fail -L -o cache/$*/kubectl-$(KUBECTL_VERSION).tar.gz "https://dl.k8s.io/$(KUBECTL_VERSION)/kubernetes-client-$*.tar.gz"
	[ $* != "linux-$(ARCH)" ] || echo "$(KUBECTL_CHECKSUM_$(ARCH))  cache/$*/kubectl-$(KUBECTL_VERSION).tar.gz" | shasum -a 512 -c
	tar -m --strip-components 3 -C ./cache/$* -xzf cache/$*/kubectl-$(KUBECTL_VERSION).tar.gz kubernetes/client/bin/kubectl
	mv ./cache/$*/kubectl $@

cache/%/kustomize-$(KUSTOMIZE_VERSION): docker/kustomize.version
	mkdir -p cache/$*
	curl --fail -L -o $@ "https://github.com/kubernetes-sigs/kustomize/releases/download/v$(KUSTOMIZE_VERSION)/kustomize_$(KUSTOMIZE_VERSION)_`echo $* | tr - _`"
	[ $* != "linux-amd64" ] || echo "$(KUSTOMIZE_CHECKSUM)  $@" | shasum -a 256 -c

cache/%/helm-$(HELM_VERSION): docker/helm.version
	mkdir -p cache/$*
	curl --fail -L -o cache/$*/helm-$(HELM_VERSION).tar.gz "https://storage.googleapis.com/kubernetes-helm/helm-v$(HELM_VERSION)-$*.tar.gz"
	[ $* != "linux-$(ARCH)" ] || echo "$(HELM_CHECKSUM_$(ARCH))  cache/$*/helm-$(HELM_VERSION).tar.gz" | shasum -a 256 -c
	tar -m -C ./cache -xzf cache/$*/helm-$(HELM_VERSION).tar.gz $*/helm
	mv cache/$*/helm $@

$(GOBIN)/fluxctl: $(FLUXCTL_DEPS)
	go install ./cmd/fluxctl

$(GOBIN)/fluxd: $(FLUXD_DEPS)
	go install ./cmd/fluxd

integration-test: all
	test/bin/test-flux



generate-deploy: install/generated_templates.gogen.go
	cd deploy && go run ../install/generate.go deploy

install/generated_templates.gogen.go: install/templates/*
	cd install && go run generate.go embedded-templates

check-generated: generate-deploy install/generated_templates.gogen.go
	git diff --exit-code -- integrations/apis integrations/client install/generated_templates.gogen.go

build-docs:
	@cd docs && docker build -t flux-docs .

test-docs: build-docs
	@docker run -it flux-docs /usr/bin/linkchecker _build/html/index.html

serve-docs: build-docs
	@echo Stating docs website on http://localhost:${DOCS_PORT}/_build/html/index.html
	@docker run -i -p ${DOCS_PORT}:8000 -e USER_ID=$$UID flux-docs
