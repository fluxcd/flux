package kubernetes

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
)

func TestKnownCRDScope(t *testing.T) {
	coreClient := makeFakeClient()

	nser, err := NewNamespacer(coreClient.Discovery())
	if err != nil {
		t.Fatal(err)
	}
	manifests := Manifests{nser}

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

	if err = ioutil.WriteFile(filepath.Join(dir, "test.yaml"), []byte(defs), 0600); err != nil {
		t.Fatal(err)
	}

	resources, err := manifests.LoadManifests(dir, []string{dir})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := resources["bar:foo/fooinstance"]; !ok {
		t.Fatal("couldn't find crd instance")
	}

}
