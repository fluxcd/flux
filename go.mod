module github.com/fluxcd/flux

go 1.16

// transitive requirement from Helm Operator
replace (
	github.com/docker/distribution => github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/docker => github.com/docker/docker v0.7.3-0.20190327010347-be7ac8be2ae0
	github.com/fluxcd/helm-operator => github.com/fluxcd/helm-operator v1.4.0
	github.com/fluxcd/helm-operator/pkg/install => github.com/fluxcd/helm-operator/pkg/install v0.0.0-20200213151218-f7e487142b46
)

// dgrijalva/jwt-go is no longer maintained, replacing with a fork which is community maintained.
replace github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt v3.2.2+incompatible

// Pin kubernetes dependencies to 1.21.14
replace (
	k8s.io/api => k8s.io/api v0.21.14
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.14
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.14
	k8s.io/client-go => k8s.io/client-go v0.21.14
	k8s.io/code-generator => k8s.io/code-generator v0.21.14
)

// Versions v3.0.0>x>v2.4.0 are susceptible to CVE-2022-28948
replace gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.1

// Fix CVE-2022-1996
replace github.com/emicklei/go-restful => github.com/emicklei/go-restful v2.16.0+incompatible

// Version v1.5.0 breaks make test
replace github.com/spf13/cobra => github.com/spf13/cobra v1.4.0

// github.com/fluxcd/flux/pkg/install lives in this very repository, so use that
replace github.com/fluxcd/flux/pkg/install => ./pkg/install

require (
	github.com/Jeffail/gabs v1.4.0
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aws/aws-sdk-go v1.44.61
	github.com/bradfitz/gomemcache v0.0.0-20220106215444-fb4bf637b56d
	github.com/cheggaaa/pb/v3 v3.1.0
	github.com/containerd/continuity v0.3.0 // indirect
	github.com/docker/distribution v2.8.1+incompatible
	github.com/evanphx/json-patch v5.6.0+incompatible
	github.com/fluxcd/flux/pkg/install v0.0.0-00010101000000-000000000000
	github.com/fluxcd/helm-operator v1.4.2
	github.com/ghodss/yaml v1.0.0
	github.com/go-kit/kit v0.12.0
	github.com/golang/gddo v0.0.0-20210115222349-20d68f94ee1f
	github.com/google/go-containerregistry v0.11.0
	github.com/google/go-github/v28 v28.1.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.5.0
	github.com/imdario/mergo v0.3.13
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/runc v1.1.4 // indirect
	github.com/opentracing-contrib/go-stdlib v1.0.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pkg/term v1.1.0
	github.com/prometheus/client_golang v1.12.2
	github.com/prometheus/client_model v0.2.0
	github.com/ryanuber/go-glob v1.0.0
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.0
	// Latest versions of github.com/weaveworks/common are not supporterd by Flux.
	github.com/weaveworks/common v0.0.0-20190410110702-87611edc252e
	github.com/whilp/git-urls v1.0.0
	github.com/xeipuuv/gojsonschema v1.2.0
	go.mozilla.org/sops/v3 v3.7.3
	golang.org/x/crypto v0.0.0-20220427172511-eb4f295cb31f // indirect
	golang.org/x/oauth2 v0.0.0-20220722155238-128564f6959c
	golang.org/x/sync v0.0.0-20220819030929-7fc1605a5dde // indirect
	golang.org/x/sys v0.0.0-20220825204002-c680a09ffe64
	golang.org/x/time v0.0.0-20220722155302-e5dcc9cfc0b9
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.14
	k8s.io/apiextensions-apiserver v0.21.14
	k8s.io/apimachinery v0.21.14
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v1.0.0
)
