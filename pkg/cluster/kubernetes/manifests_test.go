package kubernetes

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/fluxcd/flux/pkg/cluster/kubernetes/testfiles"
	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
)

func TestLocalCRDScope(t *testing.T) {
	manifests := NewManifests(log.NewLogfmtLogger(os.Stdout))

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

	err := ioutil.WriteFile(filepath.Join(dir, "test.yaml"), []byte(defs), 0600)
	assert.NoError(t, err)

	resources, err := manifests.LoadManifests(dir, []string{dir})
	if err != nil {
		t.Fatal(err)
	}

	assert.Contains(t, resources, "bar:foo/fooinstance")
}
