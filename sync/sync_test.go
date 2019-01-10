package sync

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/cluster/kubernetes"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/resource"
)

// Test that cluster.Sync gets called with the appropriate things when
// run.
func TestSync(t *testing.T) {
	checkout, cleanup := setup(t)
	defer cleanup()

	// Start with nothing running. We should be told to apply all the things.
	manifests := &kubernetes.Manifests{}
	clus := &syncCluster{map[string][]byte{}}

	dirs := checkout.ManifestDirs()
	resources, err := manifests.LoadManifests(checkout.Dir(), dirs)
	if err != nil {
		t.Fatal(err)
	}

	if err := Sync(resources, clus); err != nil {
		t.Fatal(err)
	}
	checkClusterMatchesFiles(t, manifests, clus, checkout.Dir(), dirs)
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

type syncCluster struct{ resources map[string][]byte }

func (p *syncCluster) Sync(def cluster.SyncDef) error {
	println("=== Syncing ===")
	for _, stack := range def.Stacks {
		for _, resource := range stack.Resources {
			println("Applying " + resource.ResourceID().String())
			p.resources[resource.ResourceID().String()] = resource.Bytes()
		}
	}
	println("=== Done syncing ===")
	return nil
}

func (p *syncCluster) Export() ([]byte, error) {
	// We need a response for Export, which is supposed to supply the
	// entire configuration as a lump of bytes.
	var configs [][]byte
	for _, config := range p.resources {
		configs = append(configs, config)
	}
	return bytes.Join(configs, []byte("\n---\n")), nil
}

func resourcesToStrings(resources map[string]resource.Resource) map[string]string {
	res := map[string]string{}
	for k, r := range resources {
		res[k] = string(r.Bytes())
	}
	return res
}

type SyncExporter interface {
	Syncer
	Export() ([]byte, error)
}

// Our invariant is that the model we can export from the cluster
// should always reflect what's in git. So, let's check that.
func checkClusterMatchesFiles(t *testing.T, m cluster.Manifests, c SyncExporter, base string, dirs []string) {
	conf, err := c.Export()
	if err != nil {
		t.Fatal(err)
	}
	resources, err := m.ParseManifests(conf)
	if err != nil {
		t.Fatal(err)
	}
	files, err := m.LoadManifests(base, dirs)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, resourcesToStrings(resources), resourcesToStrings(files))
}
