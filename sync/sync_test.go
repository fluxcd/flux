package sync

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"

	//	"github.com/weaveworks/flux"
	"context"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/cluster/kubernetes"
	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/resource"
)

func TestSync(t *testing.T) {
	checkout, cleanup := setup(t)
	defer cleanup()

	// Let's test that platform.Sync gets called with the appropriate
	// things when we add and remove resources from the config.

	// Start with nothing running. We should be told to apply all the things.
	mockCluster := &cluster.Mock{}
	manifests := &kubernetes.Manifests{}
	var clus cluster.Cluster = &syncCluster{mockCluster, map[string][]byte{}}

	resources, err := manifests.LoadManifests(checkout.ManifestDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := Sync(manifests, resources, clus, true, log.NewNopLogger()); err != nil {
		t.Fatal(err)
	}
	checkClusterMatchesFiles(t, manifests, clus, checkout.ManifestDir())

	for file := range testfiles.Files {
		if err := execCommand("rm", filepath.Join(checkout.ManifestDir(), file)); err != nil {
			t.Fatal(err)
		}
		commitAction := &git.CommitAction{Author: "", Message: "deleted " + file}
		if err := checkout.CommitAndPush(context.Background(), commitAction, nil); err != nil {
			t.Fatal(err)
		}
		break
	}

	resources, err = manifests.LoadManifests(checkout.ManifestDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := Sync(manifests, resources, clus, true, log.NewNopLogger()); err != nil {
		t.Fatal(err)
	}
	checkClusterMatchesFiles(t, manifests, clus, checkout.ManifestDir())
}

func TestSeparateByType(t *testing.T) {
	var tests = []struct {
		msg            string
		resMap         map[string]resource.Resource
		expectedNS     map[string]resource.Resource
		expectedOthers map[string]resource.Resource
	}{
		{
			msg:            "No resources",
			resMap:         make(map[string]resource.Resource),
			expectedNS:     nil,
			expectedOthers: nil,
		}, {
			msg: "Only namespace resources",
			resMap: map[string]resource.Resource{
				"res1": mockResourceWithoutIgnorePolicy("namespace", "ns1", "ns1"),
				"res2": mockResourceWithoutIgnorePolicy("namespace", "ns2", "ns2"),
				"res3": mockResourceWithoutIgnorePolicy("namespace", "ns3", "ns3"),
			},
			expectedNS: map[string]resource.Resource{
				"res1": mockResourceWithoutIgnorePolicy("namespace", "ns1", "ns1"),
				"res2": mockResourceWithoutIgnorePolicy("namespace", "ns2", "ns2"),
				"res3": mockResourceWithoutIgnorePolicy("namespace", "ns3", "ns3"),
			},
			expectedOthers: make(map[string]resource.Resource),
		}, {
			msg: "Only non-namespace resources",
			resMap: map[string]resource.Resource{
				"res1": mockResourceWithoutIgnorePolicy("deployment", "default", "ns1"),
				"res2": mockResourceWithoutIgnorePolicy("deployment", "ns1", "ns2"),
				"res3": mockResourceWithoutIgnorePolicy("deployment", "ns2", "ns3"),
			},
			expectedNS: make(map[string]resource.Resource),
			expectedOthers: map[string]resource.Resource{
				"res1": mockResourceWithoutIgnorePolicy("deployment", "default", "ns1"),
				"res2": mockResourceWithoutIgnorePolicy("deployment", "ns1", "ns2"),
				"res3": mockResourceWithoutIgnorePolicy("deployment", "ns2", "ns3"),
			},
		}, {
			msg: "Mixture of resources",
			resMap: map[string]resource.Resource{
				"res1": mockResourceWithoutIgnorePolicy("namespace", "ns1", "ns1"),
				"res2": mockResourceWithoutIgnorePolicy("namespace", "ns2", "ns2"),
				"res3": mockResourceWithoutIgnorePolicy("deployment", "default", "ns1"),
				"res4": mockResourceWithoutIgnorePolicy("secret", "ns1", "ns2"),
				"res5": mockResourceWithoutIgnorePolicy("service", "ns2", "ns2"),
			},
			expectedNS: map[string]resource.Resource{
				"res1": mockResourceWithoutIgnorePolicy("namespace", "ns1", "ns1"),
				"res2": mockResourceWithoutIgnorePolicy("namespace", "ns2", "ns2"),
			},
			expectedOthers: map[string]resource.Resource{
				"res3": mockResourceWithoutIgnorePolicy("deployment", "default", "ns1"),
				"res4": mockResourceWithoutIgnorePolicy("secret", "ns1", "ns2"),
				"res5": mockResourceWithoutIgnorePolicy("service", "ns2", "ns2"),
			},
		},
	}

	for _, sc := range tests {
		r1, r2 := separateResourcesByType(sc.resMap)

		if !reflect.DeepEqual(sc.expectedNS, r1) {
			t.Errorf("%s: expected %+v, got %+v\n", sc.msg, sc.expectedNS, r1)
		}
		if !reflect.DeepEqual(sc.expectedOthers, r2) {
			t.Errorf("%s: expected %+v, got %+v\n", sc.msg, sc.expectedOthers, r2)
		}
	}
}

func TestPrepareSyncDelete(t *testing.T) {
	var tests = []struct {
		msg      string
		repoRes  map[string]resource.Resource
		id       string
		res      resource.Resource
		expected *cluster.SyncDef
	}{
		{
			msg:      "No repo resources provided during sync delete",
			repoRes:  map[string]resource.Resource{},
			id:       "res7",
			res:      mockResourceWithIgnorePolicy("service", "ns1", "s2"),
			expected: &cluster.SyncDef{},
		},
		{
			msg: "No policy to ignore in place during sync delete",
			repoRes: map[string]resource.Resource{
				"res1": mockResourceWithoutIgnorePolicy("namespace", "ns1", "ns1"),
				"res2": mockResourceWithoutIgnorePolicy("namespace", "ns2", "ns2"),
				"res3": mockResourceWithoutIgnorePolicy("namespace", "ns3", "ns3"),
				"res4": mockResourceWithoutIgnorePolicy("deployment", "ns1", "d1"),
				"res5": mockResourceWithoutIgnorePolicy("deployment", "ns2", "d2"),
				"res6": mockResourceWithoutIgnorePolicy("service", "ns3", "s1"),
			},
			id:       "res7",
			res:      mockResourceWithIgnorePolicy("service", "ns1", "s2"),
			expected: &cluster.SyncDef{},
		},
		{
			msg: "No policy to ignore during sync delete",
			repoRes: map[string]resource.Resource{
				"res1": mockResourceWithoutIgnorePolicy("namespace", "ns1", "ns1"),
				"res2": mockResourceWithoutIgnorePolicy("namespace", "ns2", "ns2"),
				"res3": mockResourceWithoutIgnorePolicy("namespace", "ns3", "ns3"),
				"res4": mockResourceWithoutIgnorePolicy("deployment", "ns1", "d1"),
				"res5": mockResourceWithoutIgnorePolicy("deployment", "ns2", "d2"),
				"res6": mockResourceWithoutIgnorePolicy("service", "ns3", "s1"),
			},
			id:       "res7",
			res:      mockResourceWithoutIgnorePolicy("service", "ns1", "s2"),
			expected: &cluster.SyncDef{Actions: []cluster.SyncAction{cluster.SyncAction{ResourceID: "res7", Delete: cluster.ResourceDef{}, Apply: cluster.ResourceDef(nil)}}},
		},
	}

	logger := log.NewNopLogger()
	for _, sc := range tests {
		sync := &cluster.SyncDef{}
		prepareSyncDelete(logger, sc.repoRes, sc.id, sc.res, sync)

		if !reflect.DeepEqual(sc.expected, sync) {
			t.Errorf("%s: expected %+v, got %+v\n", sc.msg, sc.expected, sync)
		}
	}
}

func TestPrepareSyncApply(t *testing.T) {
	var tests = []struct {
		msg      string
		clusRes  map[string]resource.Resource
		id       string
		res      resource.Resource
		expected *cluster.SyncDef
	}{
		{
			msg:      "No repo resources provided during sync apply",
			clusRes:  map[string]resource.Resource{},
			id:       "res1",
			res:      mockResourceWithIgnorePolicy("service", "ns1", "s2"),
			expected: &cluster.SyncDef{},
		},
		{
			msg: "No policy to ignore in place during sync apply",
			clusRes: map[string]resource.Resource{
				"res1": mockResourceWithoutIgnorePolicy("namespace", "ns1", "ns1"),
				"res2": mockResourceWithoutIgnorePolicy("namespace", "ns2", "ns2"),
				"res3": mockResourceWithoutIgnorePolicy("namespace", "ns3", "ns3"),
				"res4": mockResourceWithoutIgnorePolicy("deployment", "ns1", "d1"),
				"res5": mockResourceWithoutIgnorePolicy("deployment", "ns2", "d2"),
				"res6": mockResourceWithoutIgnorePolicy("service", "ns3", "s1"),
			},
			id:       "res7",
			res:      mockResourceWithIgnorePolicy("service", "ns1", "s2"),
			expected: &cluster.SyncDef{},
		},
		{
			msg: "No policy to ignore during sync apply",
			clusRes: map[string]resource.Resource{
				"res1": mockResourceWithoutIgnorePolicy("namespace", "ns1", "ns1"),
				"res2": mockResourceWithoutIgnorePolicy("namespace", "ns2", "ns2"),
				"res3": mockResourceWithoutIgnorePolicy("namespace", "ns3", "ns3"),
				"res4": mockResourceWithoutIgnorePolicy("deployment", "ns1", "d1"),
				"res5": mockResourceWithoutIgnorePolicy("deployment", "ns2", "d2"),
				"res6": mockResourceWithoutIgnorePolicy("service", "ns3", "s1"),
			},
			id:       "res7",
			res:      mockResourceWithoutIgnorePolicy("service", "ns1", "s2"),
			expected: &cluster.SyncDef{Actions: []cluster.SyncAction{cluster.SyncAction{ResourceID: "res7", Apply: cluster.ResourceDef{}, Delete: cluster.ResourceDef(nil)}}},
		},
	}

	logger := log.NewNopLogger()
	for _, sc := range tests {
		sync := &cluster.SyncDef{}
		prepareSyncApply(logger, sc.clusRes, sc.id, sc.res, sync)

		if !reflect.DeepEqual(sc.expected, sync) {
			t.Errorf("%s: expected %+v, got %+v\n", sc.msg, sc.expected, sync)
		}
	}
}

// ---

var gitconf = git.Config{
	SyncTag:   "test-sync",
	NotesRef:  "test-notes",
	UserName:  "testuser",
	UserEmail: "test@example.com",
}

func setup(t *testing.T) (*git.Checkout, func()) {
	// All the mocks, mockity mock.
	repo, cleanupRepo := gittest.Repo(t)

	// Clone the repo so we can mess with the files
	working, err := repo.Clone(context.Background(), gitconf)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		cleanupRepo()
		working.Clean()
	}

	return working, cleanup
}

func execCommand(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	fmt.Printf("exec: %s %s\n", cmd, strings.Join(args, " "))
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
	return c.Run()
}

// A platform that keeps track of exactly what it's been told to apply
// or delete and parrots it back when asked to Export. This is as
// mechanically simple as possible!

type syncCluster struct {
	*cluster.Mock
	resources map[string][]byte
}

func (p *syncCluster) Sync(def cluster.SyncDef) error {
	println("=== Syncing ===")
	for _, action := range def.Actions {
		if action.Delete != nil {
			println("Deleting " + action.ResourceID)
			delete(p.resources, action.ResourceID)
		}
		if action.Apply != nil {
			println("Applying " + action.ResourceID)
			p.resources[action.ResourceID] = action.Apply
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

// Our invariant is that the model we can export from the platform
// should always reflect what's in git. So, let's check that.
func checkClusterMatchesFiles(t *testing.T, m cluster.Manifests, c cluster.Cluster, dir string) {
	conf, err := c.Export()
	if err != nil {
		t.Fatal(err)
	}
	resources, err := m.ParseManifests(conf)
	if err != nil {
		t.Fatal(err)
	}
	files, err := m.LoadManifests(dir)
	if err != nil {
		t.Fatal(err)
	}

	expected := resourcesToStrings(files)
	got := resourcesToStrings(resources)

	if !reflect.DeepEqual(expected, got) {
		t.Errorf("expected:\n%#v\ngot:\n%#v", expected, got)
	}
}
