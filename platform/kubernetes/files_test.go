package kubernetes

import (
	"reflect"
	"testing"

	"github.com/weaveworks/flux/platform/kubernetes/testdata"
)

func TestDefinedServices(t *testing.T) {
	dir, cleanup := testdata.TempDir(t)
	defer cleanup()

	if err := testdata.WriteTestFiles(dir); err != nil {
		t.Fatal(err)
	}

	services, err := FindDefinedServices(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(testdata.ServiceMap(dir), services) {
		t.Errorf("Expected:\n%#v\ngot:\n%#v\n", testdata.ServiceMap(dir), services)
	}
}
