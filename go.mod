module github.com/fluxcd/flux

go 1.13

// remove when https://github.com/docker/distribution/pull/2905 is released.
replace github.com/docker/distribution => github.com/fluxcd/distribution v0.0.0-20190419185413-6c9727e5e5de

// fix go-autorest ambiguous import caused by sops
// sops needs to update their deps ref: https://github.com/kubernetes/client-go/issues/628
replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.2.0+incompatible

// transitive requirement from Helm Operator (to be re-evaluated on helm-op 1.0.0 GA)
replace (
	github.com/docker/docker => github.com/docker/docker v0.7.3-0.20190327010347-be7ac8be2ae0
	github.com/fluxcd/helm-operator => github.com/fluxcd/helm-operator v1.0.0-rc9
	github.com/fluxcd/helm-operator/pkg/install => github.com/fluxcd/helm-operator/pkg/install v0.0.0-20200213151218-f7e487142b46
)

// Pin kubernetes dependencies to 1.17.4
replace (
	k8s.io/api => k8s.io/api v0.17.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.4
	k8s.io/client-go => k8s.io/client-go v0.17.4
	k8s.io/code-generator => k8s.io/code-generator v0.17.4
)

// github.com/fluxcd/flux/pkg/install lives in this very repository, so use that
replace github.com/fluxcd/flux/pkg/install => ./pkg/install

require (
	github.com/Jeffail/gabs v1.4.0
	github.com/Masterminds/semver/v3 v3.0.3
	github.com/aws/aws-sdk-go v1.27.1
	github.com/bradfitz/gomemcache v0.0.0-20190329173943-551aad21a668
	github.com/cheggaaa/pb/v3 v3.0.2
	github.com/docker/distribution v2.7.1+incompatible
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/fluxcd/flux/pkg/install v0.0.0-00010101000000-000000000000
	github.com/fluxcd/helm-operator v1.0.0-rc6
	github.com/ghodss/yaml v1.0.0
	github.com/go-kit/kit v0.9.0
	github.com/golang/gddo v0.0.0-20190312205958-5a2505f3dbf0
	github.com/google/go-containerregistry v0.0.0-20200121192426-b0ae1fc74a66
	github.com/google/go-github/v28 v28.1.1
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/websocket v1.4.1
	github.com/imdario/mergo v0.3.8
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opentracing-contrib/go-stdlib v0.0.0-20190519235532-cf7a6c988dc9 // indirect
	github.com/pkg/errors v0.8.1
	github.com/pkg/term v0.0.0-20190109203006-aa71e9d9e942
	github.com/prometheus/client_golang v1.2.1
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4
	github.com/ryanuber/go-glob v1.0.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	github.com/weaveworks/common v0.0.0-20190410110702-87611edc252e
	github.com/weaveworks/go-checkpoint v0.0.0-20170503165305-ebbb8b0518ab
	github.com/whilp/git-urls v0.0.0-20160530060445-31bac0d230fa
	github.com/xeipuuv/gojsonschema v1.1.0
	go.mozilla.org/sops/v3 v3.5.0
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20191028164358-195ce5e7f934
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.4
	k8s.io/apiextensions-apiserver v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v1.0.0
)
