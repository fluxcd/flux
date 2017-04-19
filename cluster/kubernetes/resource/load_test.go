package resource

import (
	"reflect"
	"testing"

	"github.com/weaveworks/flux/cluster/kubernetes/testdata"
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
kind: Service
metadata:
  name: b-service
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
	objB := base("test", "Service", "b-namespace", "b-service")
	expected := map[string]Resource{
		objA.ResourceID(): &Deployment{baseObject: objA},
		objB.ResourceID(): &Service{baseObject: objB},
	}

	for id, obj := range expected {
		if !reflect.DeepEqual(obj, objs[id]) {
			t.Errorf("At %+v expected:\n%#v\ngot:\n%#v", id, obj, objs[id])
		}
	}
}

func TestLoadSome(t *testing.T) {
	dir, cleanup := testdata.TempDir(t)
	defer cleanup()
	if err := testdata.WriteTestFiles(dir); err != nil {
		t.Fatal(err)
	}
	objs, err := Load(dir)
	if err != nil {
		t.Error(err)
	}
	// assume it's one per file for the minute
	if len(objs) != len(testdata.Files) {
		t.Errorf("expected %d objects from %d files, got result:\n%#v", len(testdata.Files), len(testdata.Files), objs)
	}
}
