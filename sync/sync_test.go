package sync

import (
	"context"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/cluster/kubernetes"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/resourcestore"
)

// Test that cluster.Sync gets called with the appropriate things when
// run.
func TestSync(t *testing.T) {
	checkout, cleanup := setup(t)
	defer cleanup()

	// Start with nothing running. We should be told to apply all the things.
	manifests := kubernetes.NewManifests(kubernetes.ConstNamespacer("default"), log.NewLogfmtLogger(os.Stdout))
	policyTranslator := &kubernetes.PolicyTranslator{}
	clus := &syncCluster{map[string]string{}}

	dirs := checkout.ManifestDirs()
	rs, err := resourcestore.NewCheckoutManager(context.TODO(), false, manifests, policyTranslator, checkout)
	if err != nil {
		t.Fatal(err)
	}
	resources, err := rs.GetAllResourcesByID()
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
	SyncTag:   "test-sync",
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
func checkClusterMatchesFiles(t *testing.T, rs resourcestore.ResourceStore, resources map[string]string, base string, dirs []string) {
	files, err := rs.GetAllResourcesByID()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, resources, resourcesToStrings(files))
}
