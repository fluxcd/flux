module github.com/fluxcd/flux

go 1.13

require (
	github.com/Jeffail/gabs v1.4.0
	github.com/Masterminds/semver v1.4.2
	github.com/argoproj/argo v2.4.2+incompatible // indirect
	github.com/argoproj/argo-cd v1.2.3
	github.com/argoproj/argo-cd/engine v0.0.0-00010101000000-000000000000
	github.com/aws/aws-sdk-go v1.19.11
	github.com/bradfitz/gomemcache v0.0.0-20190329173943-551aad21a668
	github.com/docker/distribution v0.0.0-00010101000000-000000000000
	github.com/evanphx/json-patch v4.2.0+incompatible
	github.com/fluxcd/helm-operator v1.0.0-rc2
	github.com/ghodss/yaml v1.0.0
	github.com/go-kit/kit v0.9.0
	github.com/go-redis/cache v6.4.0+incompatible // indirect
	github.com/go-redis/redis v6.15.6+incompatible // indirect
	github.com/golang/gddo v0.0.0-20190312205958-5a2505f3dbf0
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/gorilla/mux v1.7.1
	github.com/gorilla/websocket v1.4.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/imdario/mergo v0.3.8
	github.com/instrumenta/kubeval v0.0.0-20190804145309-805845b47dfc
	github.com/justinbarrick/go-k8s-portforward v1.0.4-0.20190722134107-d79fe1b9d79d
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/pkg/errors v0.8.1
	github.com/pkg/term v0.0.0-20190109203006-aa71e9d9e942
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/ryanuber/go-glob v1.0.0
	github.com/shurcooL/httpfs v0.0.0-20190707220628-8d4bc4ba7749
	github.com/shurcooL/vfsgen v0.0.0-20181202132449-6a9ea43bcacd
	github.com/spf13/cobra v0.0.4
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.4.0
	github.com/vmihailenco/msgpack v4.0.4+incompatible // indirect
	github.com/weaveworks/common v0.0.0-20190410110702-87611edc252e
	github.com/weaveworks/go-checkpoint v0.0.0-20170503165305-ebbb8b0518ab
	github.com/whilp/git-urls v0.0.0-20160530060445-31bac0d230fa
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/net v0.0.0-20190724013045-ca1201d0de80 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/sys v0.0.0-20190801041406-cbf593c0f2f3
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190816222004-e3a6b8045b0b
	k8s.io/apiextensions-apiserver v0.0.0-20190404071145-7f7d2b94eca3
	k8s.io/apimachinery v0.0.0-20190816221834-a9f1d8a9c101
	k8s.io/client-go v11.0.1-0.20190816222228-6d55c1b1f1ca+incompatible
	k8s.io/helm v2.13.1+incompatible
	k8s.io/klog v0.3.3
	k8s.io/utils v0.0.0-20190712204705-3dccf664f023 // indirect
)

replace github.com/docker/distribution => github.com/2opremio/distribution v0.0.0-20190419185413-6c9727e5e5de

// The following pin these libs to `kubernetes-1.14.4` (by initially
// giving the version as that tag, and letting go mod fill in its idea of
// the version).
// The libs are thereby kept compatible with client-go v11, which is
// itself compatible with Kubernetes 1.14.

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190708174958-539a33f6e817
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190708180123-608cd7da68f7
	k8s.io/client-go => k8s.io/client-go v11.0.0+incompatible
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190708175518-244289f83105
)

replace github.com/argoproj/argo-cd/engine => github.com/argoproj/gitops-engine v0.0.0-20191112175126-bee53e288f1c
