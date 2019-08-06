module github.com/weaveworks/flux

go 1.12

require (
	github.com/Masterminds/semver v1.4.2
	github.com/aws/aws-sdk-go v1.19.11
	github.com/bradfitz/gomemcache v0.0.0-20190329173943-551aad21a668
	github.com/docker/distribution v0.0.0-00010101000000-000000000000
	github.com/evanphx/json-patch v4.1.0+incompatible
	github.com/fluxcd/helm-operator v0.0.0-20190806135400-402019a4607f // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-kit/kit v0.9.0
	github.com/golang/gddo v0.0.0-20190312205958-5a2505f3dbf0
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.3.0
	github.com/gorilla/mux v1.7.1
	github.com/gorilla/websocket v1.4.0
	github.com/imdario/mergo v0.3.7
	github.com/instrumenta/kubeval v0.0.0-20190720105720-70e32d660927
	github.com/justinbarrick/go-k8s-portforward v1.0.4-0.20190722134107-d79fe1b9d79d
	github.com/ncabatoff/go-seq v0.0.0-20180805175032-b08ef85ed833
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/pkg/errors v0.8.1
	github.com/pkg/term v0.0.0-20190109203006-aa71e9d9e942
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829
	github.com/ryanuber/go-glob v1.0.0
	github.com/shurcooL/httpfs v0.0.0-20190707220628-8d4bc4ba7749
	github.com/shurcooL/vfsgen v0.0.0-20181202132449-6a9ea43bcacd
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.0
	github.com/weaveworks/common v0.0.0-20190410110702-87611edc252e
	github.com/weaveworks/go-checkpoint v0.0.0-20170503165305-ebbb8b0518ab
	github.com/whilp/git-urls v0.0.0-20160530060445-31bac0d230fa
	golang.org/x/sys v0.0.0-20190616124812-15dcb6c0061f
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190313235455-40a48860b5ab
	k8s.io/apiextensions-apiserver v0.0.0-20190315093550-53c4693659ed
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/code-generator v0.0.0-20190511023357-639c964206c2
	k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a
	k8s.io/helm v2.13.1+incompatible
	k8s.io/klog v0.3.3
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
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190311093542-50b561225d70
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190708175518-244289f83105
)
