package docker

import (
	"github.com/weaveworks/flux/cluster/docker/testfiles"

	"testing"
)

func TestFindDefinedServices(t *testing.T) {
	dir, cleanup := testfiles.TempDir(t)

	defer cleanup()

	if err := testfiles.WriteTestFiles(dir); err != nil {
		t.Fatal(err)
	}

	_, err := FindDefinedServices("default_swarm", dir)
	if err != nil {
		t.Error(err)
	}
}
