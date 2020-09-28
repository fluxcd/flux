package kubernetes

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/flux/pkg/cluster/kubernetes/testfiles"
	zapLogfmt "github.com/sykesm/zap-logfmt"
)

func TestLocalCRDScope(t *testing.T) {
	zap.RegisterEncoder("logfmt", func(config zapcore.EncoderConfig) (zapcore.Encoder, error) {
		enc := zapLogfmt.NewEncoder(config)
		return enc, nil
	})
	logCfg := zap.NewDevelopmentConfig()
	logCfg.Encoding = "logfmt"
	logger, _ := logCfg.Build()
	coreClient := makeFakeClient()

	nser, err := NewNamespacer(coreClient.Discovery(), "")
	assert.NoError(t, err)
	manifests := NewManifests(nser, logger)

	dir, cleanup := testfiles.TempDir(t)
	defer cleanup()
	const defs = `---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: foo
spec:
  group: foo.example.com
  names:
    kind: Foo
    listKind: FooList
    plural: foos
    shortNames:
    - foo
  scope: Namespaced
  version: v1beta1
  versions:
    - name: v1beta1
      served: true
      storage: true
---
apiVersion: foo.example.com/v1beta1
kind: Foo
metadata:
  name: fooinstance
  namespace: bar
`

	err = ioutil.WriteFile(filepath.Join(dir, "test.yaml"), []byte(defs), 0600)
	assert.NoError(t, err)

	resources, err := manifests.LoadManifests(dir, []string{dir})
	if err != nil {
		t.Fatal(err)
	}

	assert.Contains(t, resources, "bar:foo/fooinstance")
}

func TestUnKnownCRDScope(t *testing.T) {
	logFile, err := ioutil.TempFile("./", "TestUnKnownCRDScope.log")
	if err != nil {
		panic("failed to create temporary file")
	}
	defer os.Remove("./TestUnKnownCRDScope.log")
	zap.RegisterEncoder("logfmt", func(config zapcore.EncoderConfig) (zapcore.Encoder, error) {
		enc := zapLogfmt.NewEncoder(config)
		return enc, nil
	})
	logCfg := zap.NewDevelopmentConfig()
	logCfg.Encoding = "logfmt"
	logCfg.OutputPaths = []string{"./TestUnKnownCRDScope.log"}
	logger, _ := logCfg.Build()
	coreClient := makeFakeClient()

	nser, err := NewNamespacer(coreClient.Discovery(), "")
	assert.NoError(t, err)
	manifests := NewManifests(nser, logger)

	dir, cleanup := testfiles.TempDir(t)
	defer cleanup()
	const defs = `---
apiVersion: v1
kind: Namespace
metadata:
  name: mynamespace
---
apiVersion: foo.example.com/v1beta1
kind: Foo
metadata:
  name: fooinstance
  namespace: bar
`

	err = ioutil.WriteFile(filepath.Join(dir, "test.yaml"), []byte(defs), 0600)
	assert.NoError(t, err)

	resources, err := manifests.LoadManifests(dir, []string{dir})
	assert.NoError(t, err)

	// can't contain the CRD since we cannot figure out its scope
	assert.NotContains(t, resources, "bar:foo/fooinstance")

	// however, it should contain the namespace
	assert.Contains(t, resources, "<cluster>:namespace/mynamespace")

	savedLog, err := ioutil.ReadFile("./TestUnKnownCRDScope.log")
	if err != nil {
		t.Error(err)
	}
	// and we should had logged a warning about it
	assert.Contains(t, string(savedLog), "cannot find scope of resource foo/fooinstance")

	// loading again shouldn't result in more warnings
	resources, err = manifests.LoadManifests(dir, []string{dir})
	assert.NoError(t, err)
	savedLogCopy, err := ioutil.ReadFile("./TestUnKnownCRDScope.log")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, string(savedLogCopy), string(savedLog))

	// But getting the scope of the CRD should result in a log saying we found the scope
	apiResourcesWithoutFoo := coreClient.Resources
	apiResource := &metav1.APIResourceList{
		GroupVersion: "foo.example.com/v1beta1",
		APIResources: []metav1.APIResource{
			{Name: "foos", SingularName: "foo", Namespaced: true, Kind: "Foo"},
		},
	}
	coreClient.Resources = append(coreClient.Resources, apiResource)

	logFile.Truncate(0)
	resources, err = manifests.LoadManifests(dir, []string{dir})
	assert.NoError(t, err)
	assert.Len(t, resources, 2)
	savedLog, err = ioutil.ReadFile("./TestUnKnownCRDScope.log")
	if err != nil {
		t.Error(err)
	}
	assert.Contains(t, string(savedLog), "found scope of resource bar:foo/fooinstance")

	// and missing the scope information again should result in another warning
	coreClient.Resources = apiResourcesWithoutFoo
	logFile.Truncate(0)
	resources, err = manifests.LoadManifests(dir, []string{dir})
	assert.NoError(t, err)
	savedLog, err = ioutil.ReadFile("./TestUnKnownCRDScope.log")
	if err != nil {
		t.Error(err)
	}
	assert.Contains(t, string(savedLog), "cannot find scope of resource foo/fooinstance")
}
