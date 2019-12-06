module github.com/fluxcd/flux

go 1.13

// remove when https://github.com/docker/distribution/pull/2905 is released.
replace github.com/docker/distribution => github.com/2opremio/distribution v0.0.0-20190419185413-6c9727e5e5de

require (
	github.com/Azure/go-autorest v12.2.0+incompatible // indirect
	github.com/Jeffail/gabs v1.4.0
	github.com/Masterminds/semver v1.4.2
	github.com/aws/aws-sdk-go v1.25.48
	github.com/bradfitz/gomemcache v0.0.0-20190329173943-551aad21a668
	github.com/cheggaaa/pb/v3 v3.0.2
	github.com/docker/distribution v2.7.1+incompatible
	github.com/evanphx/json-patch v4.1.0+incompatible
	github.com/fluxcd/helm-operator v1.0.0-rc2
	github.com/ghodss/yaml v1.0.0
	github.com/go-kit/kit v0.9.0
	github.com/golang/gddo v0.0.0-20190312205958-5a2505f3dbf0
	github.com/google/go-github/v28 v28.1.1
	github.com/gorilla/mux v1.7.1
	github.com/gorilla/websocket v1.4.0
	github.com/imdario/mergo v0.3.7
	github.com/instrumenta/kubeval v0.0.0-20190804145309-805845b47dfc
	github.com/justinbarrick/go-k8s-portforward v1.0.4-0.20190722134107-d79fe1b9d79d
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/pkg/errors v0.8.1
	github.com/pkg/term v0.0.0-20190109203006-aa71e9d9e942
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/ryanuber/go-glob v1.0.0
	github.com/shurcooL/httpfs v0.0.0-20190707220628-8d4bc4ba7749
	github.com/shurcooL/vfsgen v0.0.0-20181202132449-6a9ea43bcacd
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.0
	github.com/weaveworks/common v0.0.0-20190410110702-87611edc252e
	github.com/weaveworks/go-checkpoint v0.0.0-20170503165305-ebbb8b0518ab
	github.com/whilp/git-urls v0.0.0-20160530060445-31bac0d230fa
	go.mozilla.org/sops/v3 v3.5.0
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20190801041406-cbf593c0f2f3
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190708174958-539a33f6e817 // kubernetes-1.14.4
	k8s.io/apiextensions-apiserver v0.0.0-20190315093550-53c4693659ed // kubernetes-1.14.4
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d // kubernetes-1.14.4
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/helm v2.13.1+incompatible
	k8s.io/klog v0.3.3
)
