.DEFAULT: all
.PHONY: all clean realclean

DOCKER?=docker
include docker/kubectl.version

# NB because this outputs absolute file names, you have to be careful
# if you're testing out the Makefile with `-W` (pretend a file is
# new); use the full path to the pretend-new file, e.g.,
#  `make -W $PWD/registry/registry.go`
godeps=$(shell go list -f '{{join .Deps "\n"}}' $1 | grep -v /vendor/ | xargs go list -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}')

FLUXD_DEPS:=$(call godeps,./cmd/fluxd)
FLUXSVC_DEPS:=$(call godeps,./cmd/fluxsvc)
FLUXCTL_DEPS:=$(call godeps,./cmd/fluxctl)

MIGRATIONS:=$(shell find db/migrations -type f)

all: $(GOPATH)/bin/fluxctl $(GOPATH)/bin/fluxd $(GOPATH)/bin/fluxsvc build/.fluxd.done build/.fluxsvc.done

clean:
	go clean
	rm -rf ./build

realclean: clean
	rm -rf ./cache

build/migrations.tar: $(MIGRATIONS)
	tar cf $@ db/migrations

build/.%.done: docker/Dockerfile.%
	mkdir -p ./build/docker/$*
	cp $^ ./build/docker/$*/
	${DOCKER} build -t weaveworks/$* -f build/docker/$*/Dockerfile.$* ./build/docker/$*
	touch $@

build/.fluxd.done: build/fluxd build/kubectl
build/.fluxsvc.done: build/fluxsvc cmd/fluxsvc/kubeservice build/migrations.tar

build/fluxd: $(FLUXD_DEPS)
build/fluxd: cmd/fluxd/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ cmd/fluxd/main.go

build/fluxsvc: $(FLUXSVC_DEPS)
build/fluxsvc: cmd/fluxsvc/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ cmd/fluxsvc/main.go

build/kubectl: cache/kubectl-$(KUBECTL_VERSION) docker/kubectl.version
	cp cache/kubectl-$(KUBECTL_VERSION) $@
	chmod a+x $@

cache/kubectl-$(KUBECTL_VERSION):
	mkdir -p cache
	curl -L -o $@ "https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl"

${GOPATH}/bin/fluxctl: $(FLUXCTL_DEPS)
${GOPATH}/bin/fluxctl: ./cmd/fluxctl/*.go
	go install ./cmd/fluxctl

$(GOPATH)/bin/fluxd: $(FLUXD_DEPS)
$(GOPATH)/bin/fluxd: cmd/fluxd/*.go
	go install ./cmd/fluxd

$(GOPATH)/bin/fluxsvc: $(FLUXSVC_DEPS)
$(GOPATH)/bin/fluxsvc: cmd/fluxsvc/*.go
	go install ./cmd/fluxsvc
