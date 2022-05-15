module github.com/fluxcd/flux

go 1.16

// remove when https://github.com/docker/distribution/pull/2905 is released.
// Update: on 2021-02-25 this has been merged, 2.7.2 should include it soon!
replace github.com/docker/distribution => github.com/fluxcd/distribution v0.0.0-20190419185413-6c9727e5e5de

// fix go-autorest ambiguous import caused by sops
// sops needs to update their deps ref: https://github.com/kubernetes/client-go/issues/628
// replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.2.0+incompatible

// transitive requirement from Helm Operator
replace (
	github.com/docker/docker => github.com/docker/docker v0.7.3-0.20190327010347-be7ac8be2ae0
	github.com/fluxcd/helm-operator => github.com/fluxcd/helm-operator v1.4.0
	github.com/fluxcd/helm-operator/pkg/install => github.com/fluxcd/helm-operator/pkg/install v0.0.0-20200213151218-f7e487142b46
)

// dgrijalva/jwt-go is no longer maintained, replacing with a fork which is community maintained.
replace github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt v3.2.2+incompatible

// Pin kubernetes dependencies to 1.21.3
replace (
	k8s.io/api => k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.3
	k8s.io/client-go => k8s.io/client-go v0.21.3
	k8s.io/code-generator => k8s.io/code-generator v0.21.3
)

// github.com/fluxcd/flux/pkg/install lives in this very repository, so use that
replace github.com/fluxcd/flux/pkg/install => ./pkg/install

require (
	github.com/Azure/azure-sdk-for-go v38.0.0+incompatible // indirect
	github.com/Jeffail/gabs v1.4.0
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aws/aws-sdk-go v1.43.27
	github.com/bradfitz/gomemcache v0.0.0-20220106215444-fb4bf637b56d
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/docker/distribution v2.8.1+incompatible
	github.com/evanphx/json-patch v4.11.0+incompatible
	github.com/fluxcd/flux/pkg/install v0.0.0-00010101000000-000000000000
	github.com/fluxcd/helm-operator v1.4.2
	github.com/ghodss/yaml v1.0.0
	github.com/go-kit/kit v0.12.0
	github.com/golang/gddo v0.0.0-20210115222349-20d68f94ee1f
	github.com/google/go-containerregistry v0.8.0
	github.com/google/go-github/v28 v28.1.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.5.0
	github.com/imdario/mergo v0.3.12
	github.com/opencontainers/go-digest v1.0.0
	github.com/opentracing-contrib/go-stdlib v1.0.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pkg/term v1.1.0
	github.com/prometheus/client_golang v1.12.2
	github.com/prometheus/client_model v0.2.0
	github.com/ryanuber/go-glob v1.0.0
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.1
	github.com/weaveworks/common v0.0.0-20190410110702-87611edc252e
	github.com/weaveworks/go-checkpoint v0.0.0-20220223124739-fd9899e2b4f2
	github.com/whilp/git-urls v1.0.0
	github.com/xeipuuv/gojsonschema v1.2.0
	go.mozilla.org/sops/v3 v3.7.2
	golang.org/x/crypto v0.0.0-20220427172511-eb4f295cb31f // indirect
	golang.org/x/oauth2 v0.0.0-20220309155454-6242fa91716a
	golang.org/x/sys v0.0.0-20220328115105-d36c6a25d886
	golang.org/x/time v0.0.0-20220224211638-0e9765cccd65
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v1.0.0
)
