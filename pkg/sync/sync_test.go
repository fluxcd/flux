package sync

import (
	"context"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/cluster/kubernetes"
	"github.com/fluxcd/flux/pkg/git"
	"github.com/fluxcd/flux/pkg/git/gittest"
	"github.com/fluxcd/flux/pkg/manifests"
	"github.com/fluxcd/flux/pkg/resource"
)

// Test that cluster.Sync gets called with the appropriate things when
// run.
func TestSync(t *testing.T) {
	checkout, cleanup := setup(t)
	defer cleanup()

	// Start with nothing running. We should be told to apply all the things.
	parser := kubernetes.NewManifests(kubernetes.ConstNamespacer("default"), log.NewLogfmtLogger(os.Stdout))
	clus := &syncCluster{map[string]string{}}

	dirs := checkout.AbsolutePaths()
	rs := manifests.NewRawFiles(checkout.Dir(), dirs, parser)
	resources, err := rs.GetAllResourcesByID(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	if err := Sync("synctest", resources, clus); err != nil {
		t.Fatal(err)
	}
	checkClusterMatchesFiles(t, rs, clus.resources, checkout.Dir(), dirs)
}

// ---

var gitconf = git.Config{
	NotesRef:  "test-notes",
	UserName:  "testuser",
	UserEmail: "test@example.com",
}

func setup(t *testing.T) (*git.Checkout, func()) {
	return gittest.Checkout(t)
}

// A cluster that keeps track of exactly what it's been told to apply
// or delete and parrots it back when asked to Export. This is as
// mechanically simple as possible.

type syncCluster struct{ resources map[string]string }

func (p *syncCluster) Sync(def cluster.SyncSet) error {
	println("=== Syncing ===")
	for _, resource := range def.Resources {
		println("Applying " + resource.ResourceID().String())
		p.resources[resource.ResourceID().String()] = string(resource.Bytes())
	}
	println("=== Done syncing ===")
	return nil
}

func resourcesToStrings(resources map[string]resource.Resource) map[string]string {
	res := map[string]string{}
	for k, r := range resources {
		res[k] = string(r.Bytes())
	}
	return res
}

// Our invariant is that the model we can export from the cluster
// should always reflect what's in git. So, let's check that.
func checkClusterMatchesFiles(t *testing.T, ms manifests.Store, resources map[string]string, base string, dirs []string) {
	files, err := ms.GetAllResourcesByID(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, resources, resourcesToStrings(files))
}
