package resource

import (
	"reflect"
	"testing"

	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"github.com/weaveworks/flux/resource"
)

// for convenience
func base(source, kind, namespace, name string) baseObject {
	b := baseObject{source: source, Kind: kind}
	b.Meta.Namespace = namespace
	b.Meta.Name = name
	return b
}

func TestParseEmpty(t *testing.T) {
	doc := ``

	objs, err := ParseMultidoc([]byte(doc), "test")
	if err != nil {
		t.Error(err)
	}
	if len(objs) != 0 {
		t.Errorf("expected empty set; got %#v", objs)
	}
}

func TestParseSome(t *testing.T) {
	docs := `---
kind: Deployment
metadata:
  name: b-deployment
  namespace: b-namespace
---
kind: Deployment
metadata:
  name: a-deployment
`
	objs, err := ParseMultidoc([]byte(docs), "test")
	if err != nil {
		t.Error(err)
	}

	objA := base("test", "Deployment", "", "a-deployment")
	objB := base("test", "Deployment", "b-namespace", "b-deployment")
	expected := map[string]resource.Resource{
		objA.ResourceID().String(): &Deployment{baseObject: objA},
		objB.ResourceID().String(): &Deployment{baseObject: objB},
	}

	for id, obj := range expected {
		// Remove the bytes, so we can compare
		if !reflect.DeepEqual(obj, debyte(objs[id])) {
			t.Errorf("At %+v expected:\n%#v\ngot:\n%#v", id, obj, objs[id])
		}
	}
}

func TestParseSomeWithComment(t *testing.T) {
	docs := `# some random comment
---
kind: Deployment
metadata:
  name: b-deployment
  namespace: b-namespace
---
kind: Deployment
metadata:
  name: a-deployment
`
	objs, err := ParseMultidoc([]byte(docs), "test")
	if err != nil {
		t.Error(err)
	}

	objA := base("test", "Deployment", "", "a-deployment")
	objB := base("test", "Deployment", "b-namespace", "b-deployment")
	expected := map[string]resource.Resource{
		objA.ResourceID().String(): &Deployment{baseObject: objA},
		objB.ResourceID().String(): &Deployment{baseObject: objB},
	}
	expectedL := len(expected)

	if len(objs) != expectedL {
		t.Errorf("expected %d objects from yaml source\n%s\n, got result: %d", expectedL, docs, len(objs))
	}

	for id, obj := range expected {
		// Remove the bytes, so we can compare
		if !reflect.DeepEqual(obj, debyte(objs[id])) {
			t.Errorf("At %+v expected:\n%#v\ngot:\n%#v", id, obj, objs[id])
		}
	}
}

func debyte(r resource.Resource) resource.Resource {
	if res, ok := r.(interface {
		debyte()
	}); ok {
		res.debyte()
	}
	return r
}

func TestLoadSome(t *testing.T) {
	dir, cleanup := testfiles.TempDir(t)
	defer cleanup()
	if err := testfiles.WriteTestFiles(dir); err != nil {
		t.Fatal(err)
	}
	objs, err := Load(dir)
	if err != nil {
		t.Error(err)
	}
	// assume it's one per file for the minute
	if len(objs) != len(testfiles.Files) {
		t.Errorf("expected %d objects from %d files, got result:\n%#v", len(testfiles.Files), len(testfiles.Files), objs)
	}
}
