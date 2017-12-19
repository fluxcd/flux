package daemon

import (
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log"

	"context"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/cluster/kubernetes"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/job"
	registryMock "github.com/weaveworks/flux/registry/mock"
	"github.com/weaveworks/flux/resource"
)

const (
	gitSyncTag  = "flux-sync"
	gitNotesRef = "flux"
	gitUser     = "Weave Flux"
	gitEmail    = "support@weave.works"
)

var (
	k8s    *cluster.Mock
	events *mockEventWriter
)

func daemon(t *testing.T) (*Daemon, func()) {
	repo, repoCleanup := gittest.Repo(t)
	working, err := repo.Clone(context.Background(), git.Config{
		SyncTag:   gitSyncTag,
		NotesRef:  gitNotesRef,
		UserName:  gitUser,
		UserEmail: gitEmail,
	})
	if err != nil {
		t.Fatal(err)
	}

	k8s = &cluster.Mock{}
	k8s.LoadManifestsFunc = kresource.Load
	k8s.ParseManifestsFunc = func(allDefs []byte) (map[string]resource.Resource, error) {
		return kresource.ParseMultidoc(allDefs, "exported")
	}
	k8s.ExportFunc = func() ([]byte, error) { return nil, nil }
	k8s.FindDefinedServicesFunc = (&kubernetes.Manifests{}).FindDefinedServices
	k8s.ServicesWithPoliciesFunc = (&kubernetes.Manifests{}).ServicesWithPolicies

	events = &mockEventWriter{}

	wg := &sync.WaitGroup{}
	shutdown := make(chan struct{})
	jobs := job.NewQueue(shutdown, wg)
	d := &Daemon{
		Cluster:        k8s,
		Manifests:      k8s,
		Registry:       &registryMock.Registry{},
		Checkout:       working,
		Jobs:           jobs,
		JobStatusCache: &job.StatusCache{Size: 100},
		EventWriter:    events,
		Logger:         log.NewLogfmtLogger(os.Stdout),
		LoopVars:       &LoopVars{},
	}
	return d, func() {
		close(shutdown)
		wg.Wait()
		repoCleanup()
		k8s = nil
		events = nil
	}
}

func TestPullAndSync_InitialSync(t *testing.T) {
	// No tag
	// No notes
	d, cleanup := daemon(t)
	defer cleanup()

	syncCalled := 0
	var syncDef *cluster.SyncDef
	expectedServiceIDs := flux.ResourceIDs{
		flux.MustParseResourceID("default:deployment/locked-service"),
		flux.MustParseResourceID("default:deployment/test-service"),
		flux.MustParseResourceID("default:deployment/helloworld")}
	expectedServiceIDs.Sort()
	k8s.SyncFunc = func(def cluster.SyncDef) error {
		syncCalled++
		syncDef = &def
		return nil
	}

	d.doSync(log.NewLogfmtLogger(ioutil.Discard))

	// It applies everything
	if syncCalled != 1 {
		t.Errorf("Sync was not called once, was called %d times", syncCalled)
	} else if syncDef == nil {
		t.Errorf("Sync was called with a nil syncDef")
	} else if len(syncDef.Actions) != len(expectedServiceIDs) {
		t.Errorf("Sync was not called with the %d actions, was called with: %d", len(expectedServiceIDs)*2, len(syncDef.Actions))
	}

	// The emitted event has all service ids
	es, err := events.AllEvents(time.Time{}, -1, time.Time{})
	if err != nil {
		t.Error(err)
	} else if len(es) != 1 {
		t.Errorf("Unexpected events: %#v", es)
	} else if es[0].Type != event.EventSync {
		t.Errorf("Unexpected event type: %#v", es[0])
	} else {
		gotServiceIDs := es[0].ServiceIDs
		flux.ResourceIDs(gotServiceIDs).Sort()
		if !reflect.DeepEqual(gotServiceIDs, []flux.ResourceID(expectedServiceIDs)) {
			t.Errorf("Unexpected event service ids: %#v, expected: %#v", gotServiceIDs, expectedServiceIDs)
		}
	}
	// It creates the tag at HEAD
	if err := d.Checkout.Pull(context.Background()); err != nil {
		t.Errorf("pulling sync tag: %v", err)
	} else if revs, err := d.Checkout.CommitsBefore(context.Background(), gitSyncTag); err != nil {
		t.Errorf("finding revisions before sync tag: %v", err)
	} else if len(revs) <= 0 {
		t.Errorf("Found no revisions before the sync tag")
	}
}

func TestDoSync_NoNewCommits(t *testing.T) {
	// Tag exists
	d, cleanup := daemon(t)
	defer cleanup()
	if err := d.Checkout.MoveTagAndPush(context.Background(), "HEAD", "Sync pointer"); err != nil {
		t.Fatal(err)
	}

	syncCalled := 0
	var syncDef *cluster.SyncDef
	expectedServiceIDs := flux.ResourceIDs{
		flux.MustParseResourceID("default:deployment/locked-service"),
		flux.MustParseResourceID("default:deployment/test-service"),
		flux.MustParseResourceID("default:deployment/helloworld")}
	expectedServiceIDs.Sort()
	k8s.SyncFunc = func(def cluster.SyncDef) error {
		syncCalled++
		syncDef = &def
		return nil
	}

	d.doSync(log.NewLogfmtLogger(ioutil.Discard))

	// It applies everything
	if syncCalled != 1 {
		t.Errorf("Sync was not called once, was called %d times", syncCalled)
	} else if syncDef == nil {
		t.Errorf("Sync was called with a nil syncDef")
	} else if len(syncDef.Actions) != len(expectedServiceIDs) {
		t.Errorf("Sync was not called with the %d actions, was called with: %d", len(expectedServiceIDs)*2, len(syncDef.Actions))
	}

	// The emitted event has no service ids
	es, err := events.AllEvents(time.Time{}, -1, time.Time{})
	if err != nil {
		t.Error(err)
	} else if len(es) != 0 {
		t.Errorf("Unexpected events: %#v", es)
	}

	// It doesn't move the tag
	oldRevs, err := d.Checkout.CommitsBefore(context.Background(), gitSyncTag)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Checkout.Pull(context.Background()); err != nil {
		t.Errorf("pulling sync tag: %v", err)
	} else if revs, err := d.Checkout.CommitsBefore(context.Background(), gitSyncTag); err != nil {
		t.Errorf("finding revisions before sync tag: %v", err)
	} else if !reflect.DeepEqual(revs, oldRevs) {
		t.Errorf("Should have kept the sync tag at HEAD")
	}
}

func TestDoSync_WithNewCommit(t *testing.T) {
	// Tag exists
	d, cleanup := daemon(t)
	defer cleanup()
	// Set the sync tag to head
	if err := d.Checkout.MoveTagAndPush(context.Background(), "HEAD", "Sync pointer"); err != nil {
		t.Fatal(err)
	}
	oldRevision, err := d.Checkout.HeadRevision(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Push some new changes
	if err := cluster.UpdateManifest(k8s, d.Checkout.ManifestDir(), flux.MustParseResourceID("default:deployment/helloworld"), func(def []byte) ([]byte, error) {
		// A simple modification so we have changes to push
		return []byte(strings.Replace(string(def), "replicas: 5", "replicas: 4", -1)), nil
	}); err != nil {
		t.Fatal(err)
	}

	commitAction := &git.CommitAction{Author: "", Message: "test commit"}
	if err := d.Checkout.CommitAndPush(context.Background(), commitAction, nil); err != nil {
		t.Fatal(err)
	}
	newRevision, err := d.Checkout.HeadRevision(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	syncCalled := 0
	var syncDef *cluster.SyncDef
	expectedServiceIDs := flux.ResourceIDs{
		flux.MustParseResourceID("default:deployment/locked-service"),
		flux.MustParseResourceID("default:deployment/test-service"),
		flux.MustParseResourceID("default:deployment/helloworld")}
	expectedServiceIDs.Sort()
	k8s.SyncFunc = func(def cluster.SyncDef) error {
		syncCalled++
		syncDef = &def
		return nil
	}

	d.doSync(log.NewLogfmtLogger(ioutil.Discard))

	// It applies everything
	if syncCalled != 1 {
		t.Errorf("Sync was not called once, was called %d times", syncCalled)
	} else if syncDef == nil {
		t.Errorf("Sync was called with a nil syncDef")
	} else if len(syncDef.Actions) != len(expectedServiceIDs) {
		t.Errorf("Sync was not called with the %d actions, was called with: %d", len(expectedServiceIDs)*2, len(syncDef.Actions))
	}

	// The emitted event has no service ids
	es, err := events.AllEvents(time.Time{}, -1, time.Time{})
	if err != nil {
		t.Error(err)
	} else if len(es) != 1 {
		t.Errorf("Unexpected events: %#v", es)
	} else if es[0].Type != event.EventSync {
		t.Errorf("Unexpected event type: %#v", es[0])
	} else {
		gotServiceIDs := es[0].ServiceIDs
		flux.ResourceIDs(gotServiceIDs).Sort()
		// Event should only have changed service ids
		if !reflect.DeepEqual(gotServiceIDs, []flux.ResourceID{flux.MustParseResourceID("default:deployment/helloworld")}) {
			t.Errorf("Unexpected event service ids: %#v, expected: %#v", gotServiceIDs, []flux.ResourceID{flux.MustParseResourceID("default/helloworld")})
		}
	}
	// It moves the tag
	if err := d.Checkout.Pull(context.Background()); err != nil {
		t.Errorf("pulling sync tag: %v", err)
	} else if revs, err := d.Checkout.CommitsBetween(context.Background(), oldRevision, gitSyncTag); err != nil {
		t.Errorf("finding revisions before sync tag: %v", err)
	} else if len(revs) <= 0 {
		t.Errorf("Should have moved sync tag forward")
	} else if revs[len(revs)-1].Revision != newRevision {
		t.Errorf("Should have moved sync tag to HEAD (%s), but was moved to: %s")
	}
}
