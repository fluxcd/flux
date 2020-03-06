.DEFAULT: all
.PHONY: all release-bins clean realclean test integration-test generate-deploy check-generated lint-e2e

SUDO := $(shell docker info > /dev/null 2> /dev/null || echo "sudo")

TEST_FLAGS?=

BATS_COMMIT := 3a1c2f28be260f8687ff83183cef4963faabedd6
SHELLCHECK_VERSION := 0.7.0
SHFMT_VERSION := 2.6.4
HELM_VERSION := 2.16.0

include docker/kubectl.version
include docker/kustomize.version
include docker/sops.version

# NB default target architecture is amd64. If you would like to try the
# other one -- pass an ARCH variable, e.g.,
#  `make ARCH=arm64`
ifeq ($(ARCH),)
	ARCH=amd64
endif
CURRENT_OS=$(shell go env GOOS)
CURRENT_OS_ARCH=$(shell echo $(CURRENT_OS)-`go env GOARCH`)
GOBIN?=$(shell echo `go env GOPATH`/bin)

MAIN_GO_MODULE:=$(shell go list -m -f '{{ .Path }}')
LOCAL_GO_MODULES:=$(shell go list -m -f '{{ .Path }}' all | grep $(MAIN_GO_MODULE))
godeps=$(shell go list -deps -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}' $(1) | sed "s%${PWD}/%%g")

FLUXD_DEPS:=$(call godeps,./cmd/fluxd/...)
FLUXCTL_DEPS:=$(call godeps,./cmd/fluxctl/...)

IMAGE_TAG:=$(shell ./docker/image-tag)
VCS_REF:=$(shell git rev-parse HEAD)
BUILD_DATE:=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

DOCS_PORT:=8000

GENERATED_TEMPLATES_FILE=pkg/install/generated_templates.gogen.go

all: $(GOBIN)/fluxctl $(GOBIN)/fluxd build/.flux.done

release-bins: $(GENERATED_TEMPLATES_FILE)
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
	rm -f test/bin/kubectl test/bin/helm test/bin/kind test/bin/sops test/bin/kustomize

realclean: clean
	rm -rf ./cache

test: test/bin/helm test/bin/kubectl test/bin/sops test/bin/kustomize $(GENERATED_TEMPLATES_FILE)
	PATH="${PWD}/bin:${PWD}/test/bin:${PATH}" go test ${TEST_FLAGS} $(shell go list $(patsubst %, %/..., $(LOCAL_GO_MODULES)) | sort -u)

e2e: lint-e2e test/bin/helm test/bin/kubectl test/bin/sops test/bin/crane test/e2e/bats $(GOBIN)/fluxctl build/.flux.done
	PATH="${PWD}/test/bin:${PATH}" CURRENT_OS_ARCH=$(CURRENT_OS_ARCH) test/e2e/run.bash

E2E_BATS_FILES := test/e2e/*.bats
E2E_BASH_FILES := test/e2e/run.bash test/e2e/lib/*
SHFMT_DIFF_CMD := test/bin/shfmt -i 2 -sr -d
SHFMT_WRITE_CMD := test/bin/shfmt -i 2 -sr -w
lint-e2e: test/bin/shfmt test/bin/shellcheck
	@# shfmt is not compatible with .bats files, so we preprocess them to turn '@test's into functions
	for I in $(E2E_BATS_FILES); do \
	  ( cat "$$I" | sed 's%@test.*%test() {%' | $(SHFMT_DIFF_CMD) ) || { echo "Please correct the diff for file $$I"; exit 1; }; \
	done
	$(SHFMT_DIFF_CMD) $(E2E_BASH_FILES) || ( echo "Please run '$(SHFMT_WRITE_CMD) $(E2E_BASH_FILES)'"; exit 1 )
	test/bin/shellcheck $(E2E_BASH_FILES) $(E2E_BATS_FILES)

build/.%.done: docker/Dockerfile.%
	mkdir -p ./build/docker/$*
	cp $^ ./build/docker/$*/
	$(SUDO) docker build -t docker.io/fluxcd/$* -t docker.io/fluxcd/$*:$(IMAGE_TAG) \
		--build-arg VCS_REF="$(VCS_REF)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		-f build/docker/$*/Dockerfile.$* ./build/docker/$*
	touch $@

build/.flux.done: build/fluxd build/kubectl build/sops build/kustomize docker/ssh_config docker/kubeconfig docker/known_hosts.sh

build/fluxd: $(FLUXD_DEPS)
build/fluxd: cmd/fluxd/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -o $@ $(LDFLAGS) -ldflags "-X main.version=$(shell ./docker/image-tag)" ./cmd/fluxd

build/kubectl: cache/linux-$(ARCH)/kubectl-$(KUBECTL_VERSION)
test/bin/kubectl: cache/$(CURRENT_OS_ARCH)/kubectl-$(KUBECTL_VERSION)
build/helm: cache/linux-$(ARCH)/helm-$(HELM_VERSION)
test/bin/helm: cache/$(CURRENT_OS_ARCH)/helm-$(HELM_VERSION)
build/kustomize: cache/linux-amd64/kustomize-$(KUSTOMIZE_VERSION)
build/sops: cache/linux-amd64/sops-$(SOPS_VERSION)
test/bin/kustomize: cache/$(CURRENT_OS_ARCH)/kustomize-$(KUSTOMIZE_VERSION)
test/bin/shellcheck: cache/$(CURRENT_OS_ARCH)/shellcheck-$(SHELLCHECK_VERSION)
test/bin/shfmt: cache/$(CURRENT_OS_ARCH)/shfmt-$(SHFMT_VERSION)
test/bin/sops: cache/$(CURRENT_OS_ARCH)/sops-$(SOPS_VERSION)

build/kubectl test/bin/kubectl build/kustomize test/bin/kustomize build/helm test/bin/helm test/bin/shellcheck test/bin/shfmt build/sops test/bin/sops:
	mkdir -p $(@D)
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
	curl --fail -L -o cache/$*/kustomize-$(KUSTOMIZE_VERSION).tar.gz "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv$(KUSTOMIZE_VERSION)/kustomize_v$(KUSTOMIZE_VERSION)_linux_amd64.tar.gz"
	echo "$(KUSTOMIZE_CHECKSUM)  cache/$*/kustomize-$(KUSTOMIZE_VERSION).tar.gz" | shasum -a 256 -c
	tar -m -C ./cache -xzf cache/$*/kustomize-$(KUSTOMIZE_VERSION).tar.gz kustomize
	mv cache/kustomize $@

cache/%/helm-$(HELM_VERSION):
	mkdir -p cache/$*
	curl --fail -L -o cache/$*/helm-$(HELM_VERSION).tar.gz "https://storage.googleapis.com/kubernetes-helm/helm-v$(HELM_VERSION)-$*.tar.gz"
	tar -m -C ./cache -xzf cache/$*/helm-$(HELM_VERSION).tar.gz $*/helm
	mv cache/$*/helm $@

cache/%/shellcheck-$(SHELLCHECK_VERSION):
	mkdir -p cache/$*
	curl --fail -L -o cache/$*/shellcheck-$(SHELLCHECK_VERSION).tar.xz "https://storage.googleapis.com/shellcheck/shellcheck-v$(SHELLCHECK_VERSION).$(CURRENT_OS).x86_64.tar.xz"
	tar -C cache/$* --strip-components 1 -xvJf cache/$*/shellcheck-$(SHELLCHECK_VERSION).tar.xz shellcheck-v$(SHELLCHECK_VERSION)/shellcheck
	mv cache/$*/shellcheck $@

cache/%/shfmt-$(SHFMT_VERSION):
	mkdir -p cache/$*
	curl --fail -L -o $@ "https://github.com/mvdan/sh/releases/download/v$(SHFMT_VERSION)/shfmt_v$(SHFMT_VERSION)_`echo $* | tr - _`"

cache/%/sops-$(SOPS_VERSION): docker/sops.version
	mkdir -p cache/$*
	curl --fail -L -o $@ "https://github.com/mozilla/sops/releases/download/$(SOPS_VERSION)/sops-$(SOPS_VERSION).`echo $* | cut -f1 -d"-"`"
	[ $* != "linux-amd64" ] || echo "$(SOPS_CHECKSUM)  $@" | shasum -a 256 -c

test/e2e/bats: cache/bats-core-$(BATS_COMMIT).tar.gz
	mkdir -p $@
	tar -C $@ --strip-components 1 -xzf $< 

cache/bats-core-$(BATS_COMMIT).tar.gz:
	# Use 2opremio's fork until https://github.com/bats-core/bats-core/pull/255 is merged
	curl --fail -L -o $@ https://github.com/2opremio/bats-core/archive/$(BATS_COMMIT).tar.gz

test/bin/crane:
	mkdir -p $(@D)
	go build -o $@ github.com/google/go-containerregistry/cmd/crane

$(GOBIN)/fluxctl: $(FLUXCTL_DEPS) $(GENERATED_TEMPLATES_FILE)
	go install ./cmd/fluxctl

$(GOBIN)/fluxd: $(FLUXD_DEPS)
	go install ./cmd/fluxd

generate-deploy: $(GOBIN)/fluxctl
	$(GOBIN)/fluxctl install -o ./deploy \
		--git-url git@github.com:fluxcd/flux-get-started \
		--git-email flux@example.com \
		--git-user 'Flux automation' \
		--git-label flux-sync \
		--namespace flux

$(GENERATED_TEMPLATES_FILE): pkg/install/templates/*.tmpl pkg/install/generate.go
	cd ./pkg/install &&  go generate .

check-generated: generate-deploy
	git diff --exit-code -- deploy/

build-docs:
	@cd docs && docker build -t flux-docs .

test-docs: build-docs
	@docker run -it flux-docs /usr/bin/linkchecker _build/html/index.html

serve-docs: build-docs
	@echo Stating docs website on http://localhost:${DOCS_PORT}/_build/html/index.html
	@docker run -i -p ${DOCS_PORT}:8000 -e USER_ID=$$UID flux-docs
