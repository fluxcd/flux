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
	github.com/aws/aws-sdk-go v1.40.7
	github.com/bradfitz/gomemcache v0.0.0-20190329173943-551aad21a668
	github.com/cheggaaa/pb/v3 v3.0.8
	github.com/containerd/continuity v0.0.0-20210208174643-50096c924a4e // indirect
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.3+incompatible // indirect
	github.com/evanphx/json-patch v4.11.0+incompatible
	github.com/fluxcd/flux/pkg/install v0.0.0-00010101000000-000000000000
	github.com/fluxcd/helm-operator v1.4.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-kit/kit v0.11.0
	github.com/golang/gddo v0.0.0-20190312205958-5a2505f3dbf0
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/go-containerregistry v0.5.1
	github.com/google/go-github/v28 v28.1.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/imdario/mergo v0.3.12
	github.com/opencontainers/go-digest v1.0.0
	github.com/opentracing-contrib/go-stdlib v1.0.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pkg/term v1.1.0
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/ryanuber/go-glob v1.0.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/weaveworks/common v0.0.0-20190410110702-87611edc252e
	github.com/weaveworks/go-checkpoint v0.0.0-20170503165305-ebbb8b0518ab
	github.com/whilp/git-urls v1.0.0
	github.com/xeipuuv/gojsonschema v1.2.0
	go.mozilla.org/sops/v3 v3.7.1
	golang.org/x/oauth2 v0.0.0-20210628180205-a41e5a781914
	golang.org/x/sys v0.0.0-20210817142637-7d9622a276b7
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v1.0.0
)
