package daemon

import (
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
	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/job"
	registryMock "github.com/weaveworks/flux/registry/mock"
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
	k8s.ExportFunc = func() ([]byte, error) { return nil, nil }

	events = &mockEventWriter{}

	wg := &sync.WaitGroup{}
	shutdown := make(chan struct{})

	if err := repo.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}

	gitConfig := git.Config{
		Branch:    "master",
		SyncTag:   gitSyncTag,
		NotesRef:  gitNotesRef,
		UserName:  gitUser,
		UserEmail: gitEmail,
	}

	manifests := kubernetes.NewManifests(alwaysDefault, log.NewLogfmtLogger(os.Stdout))

	jobs := job.NewQueue(shutdown, wg)
	d := &Daemon{
		Cluster:        k8s,
		Manifests:      manifests,
		Registry:       &registryMock.Registry{},
		Repo:           repo,
		GitConfig:      gitConfig,
		Jobs:           jobs,
		JobStatusCache: &job.StatusCache{Size: 100},
		EventWriter:    events,
		Logger:         log.NewLogfmtLogger(os.Stdout),
		LoopVars:       &LoopVars{GitTimeout: timeout},
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
	d, cleanup := daemon(t)
	defer cleanup()

	syncCalled := 0
	var syncDef *cluster.SyncSet
	expectedResourceIDs := flux.ResourceIDs{}
	for id, _ := range testfiles.ResourceMap {
		expectedResourceIDs = append(expectedResourceIDs, id)
	}
	expectedResourceIDs.Sort()
	k8s.SyncFunc = func(def cluster.SyncSet) error {
		syncCalled++
		syncDef = &def
		return nil
	}

	ctx := context.Background()
	head, err :=  d.Repo.BranchHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	syncTag := lastKnownSyncTag{logger: d.Logger, syncTag: d.GitConfig.SyncTag}

	if err := d.Sync(ctx, time.Now().UTC(), head, &syncTag); err != nil {
		t.Error(err)
	}

	// It applies everything
	if syncCalled != 1 {
		t.Errorf("Sync was not called once, was called %d times", syncCalled)
	} else if syncDef == nil {
		t.Errorf("Sync was called with a nil syncDef")
	}

	// The emitted event has all workload ids
	es, err := events.AllEvents(time.Time{}, -1, time.Time{})
	if err != nil {
		t.Error(err)
	} else if len(es) != 1 {
		t.Errorf("Unexpected events: %#v", es)
	} else if es[0].Type != event.EventSync {
		t.Errorf("Unexpected event type: %#v", es[0])
	} else {
		gotResourceIDs := es[0].ServiceIDs
		flux.ResourceIDs(gotResourceIDs).Sort()
		if !reflect.DeepEqual(gotResourceIDs, []flux.ResourceID(expectedResourceIDs)) {
			t.Errorf("Unexpected event workload ids: %#v, expected: %#v", gotResourceIDs, expectedResourceIDs)
		}
	}

	// It creates the tag at HEAD
	if err := d.Repo.Refresh(context.Background()); err != nil {
		t.Errorf("pulling sync tag: %v", err)
	} else if revs, err := d.Repo.CommitsBefore(context.Background(), gitSyncTag); err != nil {
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
		tagAction := git.TagAction{
			Revision: "HEAD",
			Message:  "Sync pointer",
		}
		return co.MoveSyncTagAndPush(ctx, tagAction)
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
	var syncDef *cluster.SyncSet
	expectedResourceIDs := flux.ResourceIDs{}
	for id, _ := range testfiles.ResourceMap {
		expectedResourceIDs = append(expectedResourceIDs, id)
	}
	expectedResourceIDs.Sort()
	k8s.SyncFunc = func(def cluster.SyncSet) error {
		syncCalled++
		syncDef = &def
		return nil
	}

	head, err :=  d.Repo.BranchHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	syncTag := lastKnownSyncTag{logger: d.Logger, syncTag: d.GitConfig.SyncTag}

	if err := d.Sync(ctx, time.Now().UTC(), head, &syncTag); err != nil {
		t.Error(err)
	}

	// It applies everything
	if syncCalled != 1 {
		t.Errorf("Sync was not called once, was called %d times", syncCalled)
	} else if syncDef == nil {
		t.Errorf("Sync was called with a nil syncDef")
	}

	// The emitted event has no workload ids
	es, err := events.AllEvents(time.Time{}, -1, time.Time{})
	if err != nil {
		t.Error(err)
	} else if len(es) != 0 {
		t.Errorf("Unexpected events: %#v", es)
	}

	// It doesn't move the tag
	oldRevs, err := d.Repo.CommitsBefore(ctx, gitSyncTag)
	if err != nil {
		t.Fatal(err)
	}

	if revs, err := d.Repo.CommitsBefore(ctx, gitSyncTag); err != nil {
		t.Errorf("finding revisions before sync tag: %v", err)
	} else if !reflect.DeepEqual(revs, oldRevs) {
		t.Errorf("Should have kept the sync tag at HEAD")
	}
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
		tagAction := git.TagAction{
			Revision: "HEAD",
			Message:  "Sync pointer",
		}
		err = checkout.MoveSyncTagAndPush(ctx, tagAction)
		if err != nil {
			return err
		}
		oldRevision, err = checkout.HeadRevision(ctx)
		if err != nil {
			return err
		}
		// Push some new changes
		dirs := checkout.ManifestDirs()
		err = cluster.UpdateManifest(d.Manifests, checkout.Dir(), dirs, flux.MustParseResourceID("default:deployment/helloworld"), func(def []byte) ([]byte, error) {
			// A simple modification so we have changes to push
			return []byte(strings.Replace(string(def), "replicas: 5", "replicas: 4", -1)), nil
		})
		if err != nil {
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
	var syncDef *cluster.SyncSet
	expectedResourceIDs := flux.ResourceIDs{}
	for id, _ := range testfiles.ResourceMap {
		expectedResourceIDs = append(expectedResourceIDs, id)
	}
	expectedResourceIDs.Sort()
	k8s.SyncFunc = func(def cluster.SyncSet) error {
		syncCalled++
		syncDef = &def
		return nil
	}

	head, err :=  d.Repo.BranchHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	syncTag := lastKnownSyncTag{logger: d.Logger, syncTag: d.GitConfig.SyncTag}

	if err := d.Sync(ctx, time.Now().UTC(), head, &syncTag); err != nil {
		t.Error(err)
	}

	// It applies everything
	if syncCalled != 1 {
		t.Errorf("Sync was not called once, was called %d times", syncCalled)
	} else if syncDef == nil {
		t.Errorf("Sync was called with a nil syncDef")
	}

	// The emitted event has no workload ids
	es, err := events.AllEvents(time.Time{}, -1, time.Time{})
	if err != nil {
		t.Error(err)
	} else if len(es) != 1 {
		t.Errorf("Unexpected events: %#v", es)
	} else if es[0].Type != event.EventSync {
		t.Errorf("Unexpected event type: %#v", es[0])
	} else {
		gotResourceIDs := es[0].ServiceIDs
		flux.ResourceIDs(gotResourceIDs).Sort()
		// Event should only have changed workload ids
		if !reflect.DeepEqual(gotResourceIDs, []flux.ResourceID{flux.MustParseResourceID("default:deployment/helloworld")}) {
			t.Errorf("Unexpected event workload ids: %#v, expected: %#v", gotResourceIDs, []flux.ResourceID{flux.MustParseResourceID("default:deployment/helloworld")})
		}
	}
	// It moves the tag
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := d.Repo.Refresh(ctx); err != nil {
		t.Errorf("pulling sync tag: %v", err)
	} else if revs, err := d.Repo.CommitsBetween(ctx, oldRevision, gitSyncTag); err != nil {
		t.Errorf("finding revisions before sync tag: %v", err)
	} else if len(revs) <= 0 {
		t.Errorf("Should have moved sync tag forward")
	} else if revs[len(revs)-1].Revision != newRevision {
		t.Errorf("Should have moved sync tag to HEAD (%s), but was moved to: %s", newRevision, revs[len(revs)-1].Revision)
	}
}
