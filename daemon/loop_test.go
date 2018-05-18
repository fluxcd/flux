package daemon

import (
	"io/ioutil"
	"os"
	"path/filepath"
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
	gitPath     = ""
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

	wg.Add(1)
	go repo.Start(shutdown, wg)
	gittest.WaitForRepoReady(repo, t)

	gitConfig := git.Config{
		Branch:    "master",
		SyncTag:   gitSyncTag,
		NotesRef:  gitNotesRef,
		UserName:  gitUser,
		UserEmail: gitEmail,
	}

	jobs := job.NewQueue(shutdown, wg)
	d := &Daemon{
		Cluster:        k8s,
		Manifests:      k8s,
		Registry:       &registryMock.Registry{},
		Repo:           repo,
		GitConfig:      gitConfig,
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
	if err := d.Repo.Refresh(context.Background()); err != nil {
		t.Errorf("pulling sync tag: %v", err)
	} else if revs, err := d.Repo.CommitsBefore(context.Background(), gitSyncTag, gitPath); err != nil {
		t.Errorf("finding revisions before sync tag: %v", err)
	} else if len(revs) <= 0 {
		t.Errorf("Found no revisions before the sync tag")
	}
}

func TestDoSync_NoNewCommits(t *testing.T) {
	d, cleanup := daemon(t)
	defer cleanup()

	ctx := context.Background()
	err := d.WithClone(ctx, func(co *git.Checkout) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return co.MoveSyncTagAndPush(ctx, "HEAD", "Sync pointer")
	})
	if err != nil {
		t.Fatal(err)
	}

	// NB this would usually trigger a sync in a running loop; but we
	// have not run the loop.
	if err = d.Repo.Refresh(ctx); err != nil {
		t.Error(err)
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

	if err := d.doSync(log.NewLogfmtLogger(ioutil.Discard)); err != nil {
		t.Error(err)
	}

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
	oldRevs, err := d.Repo.CommitsBefore(ctx, gitSyncTag, gitPath)
	if err != nil {
		t.Fatal(err)
	}

	if revs, err := d.Repo.CommitsBefore(ctx, gitSyncTag, gitPath); err != nil {
		t.Errorf("finding revisions before sync tag: %v", err)
	} else if !reflect.DeepEqual(revs, oldRevs) {
		t.Errorf("Should have kept the sync tag at HEAD")
	}
}

func replaceInFile(path, src, target string) error {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	replaced := strings.Replace(string(bytes), src, target, -1)
	return ioutil.WriteFile(path, []byte(replaced), os.FileMode(0666))
}

func TestDoSync_WithNewCommit(t *testing.T) {
	d, cleanup := daemon(t)
	defer cleanup()

	ctx := context.Background()
	// Set the sync tag to head
	var oldRevision, newRevision string
	err := d.WithClone(ctx, func(checkout *git.Checkout) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var err error
		err = checkout.MoveSyncTagAndPush(ctx, "HEAD", "Sync pointer")
		if err != nil {
			return err
		}
		oldRevision, err = checkout.HeadRevision(ctx)
		if err != nil {
			return err
		}
		// Push some new changes

		id := flux.MustParseResourceID("default:deployment/helloworld")
		resources, err := k8s.LoadManifests(checkout.ManifestDir(), checkout.ManifestDir())
		if err != nil {
			t.Fatal(err)
		}
		hello, ok := resources[id.String()]
		if !ok {
			t.Fatal("Did not find expected resource " + id.String())
		}
		if err = replaceInFile(filepath.Join(checkout.ManifestDir(), hello.Source()), "replicas: 5", "replicas: 4"); err != nil {
			return err
		}

		commitAction := git.CommitAction{Author: "", Message: "test commit"}
		err = checkout.CommitAndPush(ctx, commitAction, nil)
		if err != nil {
			return err
		}
		newRevision, err = checkout.HeadRevision(ctx)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	err = d.Repo.Refresh(ctx)
	if err != nil {
		t.Error(err)
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
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := d.Repo.Refresh(ctx); err != nil {
		t.Errorf("pulling sync tag: %v", err)
	} else if revs, err := d.Repo.CommitsBetween(ctx, oldRevision, gitSyncTag, gitPath); err != nil {
		t.Errorf("finding revisions before sync tag: %v", err)
	} else if len(revs) <= 0 {
		t.Errorf("Should have moved sync tag forward")
	} else if revs[len(revs)-1].Revision != newRevision {
		t.Errorf("Should have moved sync tag to HEAD (%s), but was moved to: %s", newRevision, revs[len(revs)-1].Revision)
	}
}
