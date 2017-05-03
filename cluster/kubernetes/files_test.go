package kubernetes

import (
	"reflect"
	"testing"

	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
)

func TestDefinedServices(t *testing.T) {
	dir, cleanup := testfiles.TempDir(t)
	defer cleanup()

	if err := testfiles.WriteTestFiles(dir); err != nil {
		t.Fatal(err)
	}

	services, err := (&Cluster{}).FindDefinedServices(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(testfiles.ServiceMap(dir), services) {
		t.Errorf("Expected:\n%#v\ngot:\n%#v\n", testfiles.ServiceMap(dir), services)
	}
}
