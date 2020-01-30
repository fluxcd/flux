package daemon

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/cluster/kubernetes"
	"github.com/fluxcd/flux/pkg/cluster/kubernetes/testfiles"
	"github.com/fluxcd/flux/pkg/cluster/mock"
	"github.com/fluxcd/flux/pkg/event"
	"github.com/fluxcd/flux/pkg/git"
	"github.com/fluxcd/flux/pkg/git/gittest"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/manifests"
	registryMock "github.com/fluxcd/flux/pkg/registry/mock"
	"github.com/fluxcd/flux/pkg/resource"
	fluxsync "github.com/fluxcd/flux/pkg/sync"
	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	promdto "github.com/prometheus/client_model/go"
)

const (
	gitPath     = ""
	gitNotesRef = "flux"
	gitUser     = "Flux"
	gitEmail    = "support@weave.works"
)

var (
	k8s    *mock.Mock
	events *mockEventWriter
)

func daemon(t *testing.T) (*Daemon, func()) {
	repo, repoCleanup := gittest.Repo(t)

	k8s = &mock.Mock{}
	k8s.ExportFunc = func(ctx context.Context) ([]byte, error) { return nil, nil }

	events = &mockEventWriter{}

	wg := &sync.WaitGroup{}
	shutdown := make(chan struct{})

	if err := repo.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}

	gitConfig := git.Config{
		Branch:    "master",
		NotesRef:  gitNotesRef,
		UserName:  gitUser,
		UserEmail: gitEmail,
	}

	manifests := kubernetes.NewManifests(kubernetes.ConstNamespacer("default"), log.NewLogfmtLogger(os.Stdout))

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
		LoopVars:       &LoopVars{SyncTimeout: timeout, GitTimeout: timeout},
	}
	return d, func() {
		close(shutdown)
		wg.Wait()
		repoCleanup()
		k8s = nil
		events = nil
	}
}

func findMetric(name string, metricType promdto.MetricType, labels ...string) (*promdto.Metric, error) {
	metricsRegistry := prometheus.DefaultRegisterer.(*prometheus.Registry)
	if metrics, err := metricsRegistry.Gather(); err == nil {
		for _, metricFamily := range metrics {
			if *metricFamily.Name == name {
				if *metricFamily.Type != metricType {
					return nil, fmt.Errorf("Metric types for %v doesn't correpond: %v != %v", name, metricFamily.Type, metricType)
				}
				for _, metric := range metricFamily.Metric {
					if len(labels) != len(metric.Label)*2 {
						return nil, fmt.Errorf("Metric labels length for %v doesn't correpond: %v != %v", name, len(labels)*2, len(metric.Label))
					}
					for labelIdx, label := range metric.Label {
						if labels[labelIdx*2] != *label.Name {
							return nil, fmt.Errorf("Metric label for %v doesn't correpond: %v != %v", name, labels[labelIdx*2], *label.Name)
						} else if labels[labelIdx*2+1] != *label.Value {
							break
						} else if labelIdx == len(metric.Label)-1 {
							return metric, nil
						}
					}
				}
				return nil, fmt.Errorf("Can't find metric %v with appropriate labels in registry", name)
			}
		}
		return nil, fmt.Errorf("Can't find metric %v in registry", name)
	} else {
		return nil, fmt.Errorf("Error reading metrics registry %v", err)
	}
}

func checkSyncManifestsMetrics(t *testing.T, manifestSuccess, manifestFailures int) {
	if metric, err := findMetric("flux_daemon_sync_manifests", promdto.MetricType_GAUGE, "success", "true"); err != nil {
		t.Errorf("Error collecting flux_daemon_sync_manifests{success='true'} metric: %v", err)
	} else if int(*metric.Gauge.Value) != manifestSuccess {
		t.Errorf("flux_daemon_sync_manifests{success='true'} must be %v. Got %v", manifestSuccess, *metric.Gauge.Value)
	}
	if metric, err := findMetric("flux_daemon_sync_manifests", promdto.MetricType_GAUGE, "success", "false"); err != nil {
		t.Errorf("Error collecting flux_daemon_sync_manifests{success='false'} metric: %v", err)
	} else if int(*metric.Gauge.Value) != manifestFailures {
		t.Errorf("flux_daemon_sync_manifests{success='false'} must be %v. Got %v", manifestFailures, *metric.Gauge.Value)
	}
}

func TestPullAndSync_InitialSync(t *testing.T) {
	d, cleanup := daemon(t)
	defer cleanup()

	syncCalled := 0
	var syncDef *cluster.SyncSet
	expectedResourceIDs := resource.IDs{}
	for id := range testfiles.ResourceMap {
		expectedResourceIDs = append(expectedResourceIDs, id)
	}
	expectedResourceIDs.Sort()
	k8s.SyncFunc = func(def cluster.SyncSet) error {
		syncCalled++
		syncDef = &def
		return nil
	}

	ctx := context.Background()
	head, err := d.Repo.BranchHead(ctx)
	if err != nil {
		t.Fatal(err)
	}

	syncTag := "sync"
	gitSync, _ := fluxsync.NewGitTagSyncProvider(d.Repo, syncTag, "", fluxsync.VerifySignaturesModeNone, d.GitConfig)
	syncState := &lastKnownSyncState{logger: d.Logger, state: gitSync}

	if err := d.Sync(ctx, time.Now().UTC(), head, syncState); err != nil {
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
		resource.IDs(gotResourceIDs).Sort()
		if !reflect.DeepEqual(gotResourceIDs, []resource.ID(expectedResourceIDs)) {
			t.Errorf("Unexpected event workload ids: %#v, expected: %#v", gotResourceIDs, expectedResourceIDs)
		}
	}

	// It creates the tag at HEAD
	if err := d.Repo.Refresh(context.Background()); err != nil {
		t.Errorf("pulling sync tag: %v", err)
	} else if revs, err := d.Repo.CommitsBefore(context.Background(), syncTag, false); err != nil {
		t.Errorf("finding revisions before sync tag: %v", err)
	} else if len(revs) <= 0 {
		t.Errorf("Found no revisions before the sync tag")
	}

	// Check 0 error stats
	checkSyncManifestsMetrics(t, len(expectedResourceIDs), 0)
}

func TestDoSync_NoNewCommits(t *testing.T) {
	d, cleanup := daemon(t)
	defer cleanup()

	var syncTag = "syncity"

	ctx := context.Background()
	err := d.WithWorkingClone(ctx, func(co *git.Checkout) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		tagAction := git.TagAction{
			Tag:      syncTag,
			Revision: "master",
			Message:  "Sync pointer",
		}
		return co.MoveTagAndPush(ctx, tagAction)
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
	expectedResourceIDs := resource.IDs{}
	for id := range testfiles.ResourceMap {
		expectedResourceIDs = append(expectedResourceIDs, id)
	}
	expectedResourceIDs.Sort()
	k8s.SyncFunc = func(def cluster.SyncSet) error {
		syncCalled++
		syncDef = &def
		return nil
	}

	head, err := d.Repo.BranchHead(ctx)
	if err != nil {
		t.Fatal(err)
	}

	gitSync, _ := fluxsync.NewGitTagSyncProvider(d.Repo, syncTag, "", fluxsync.VerifySignaturesModeNone, d.GitConfig)
	syncState := &lastKnownSyncState{logger: d.Logger, state: gitSync}

	if err := d.Sync(ctx, time.Now().UTC(), head, syncState); err != nil {
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
	oldRevs, err := d.Repo.CommitsBefore(ctx, syncTag, false)
	if err != nil {
		t.Fatal(err)
	}

	if revs, err := d.Repo.CommitsBefore(ctx, syncTag, false); err != nil {
		t.Errorf("finding revisions before sync tag: %v", err)
	} else if !reflect.DeepEqual(revs, oldRevs) {
		t.Errorf("Should have kept the sync tag at HEAD")
	}
}

func TestDoSync_WithNewCommit(t *testing.T) {
	d, cleanup := daemon(t)
	defer cleanup()

	ctx := context.Background()

	var syncTag = "syncy-mcsyncface"
	// Set the sync tag to head
	var oldRevision, newRevision string
	err := d.WithWorkingClone(ctx, func(checkout *git.Checkout) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var err error
		tagAction := git.TagAction{
			Tag:      syncTag,
			Revision: "master",
			Message:  "Sync pointer",
		}
		err = checkout.MoveTagAndPush(ctx, tagAction)
		if err != nil {
			return err
		}
		oldRevision, err = checkout.HeadRevision(ctx)
		if err != nil {
			return err
		}
		// Push some new changes
		cm := manifests.NewRawFiles(checkout.Dir(), checkout.AbsolutePaths(), d.Manifests)
		resourcesByID, err := cm.GetAllResourcesByID(context.TODO())
		if err != nil {
			return err
		}
		targetResource := "default:deployment/helloworld"
		res, ok := resourcesByID[targetResource]
		if !ok {
			return fmt.Errorf("resource not found: %q", targetResource)

		}
		absolutePath := path.Join(checkout.Dir(), res.Source())
		def, err := ioutil.ReadFile(absolutePath)
		if err != nil {
			return err
		}
		newDef := bytes.Replace(def, []byte("replicas: 5"), []byte("replicas: 4"), -1)
		if err := ioutil.WriteFile(absolutePath, newDef, 0600); err != nil {
			return err
		}

		commitAction := git.CommitAction{Author: "", Message: "test commit"}
		err = checkout.CommitAndPush(ctx, commitAction, nil, false)
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
	expectedResourceIDs := resource.IDs{}
	for id := range testfiles.ResourceMap {
		expectedResourceIDs = append(expectedResourceIDs, id)
	}
	expectedResourceIDs.Sort()
	k8s.SyncFunc = func(def cluster.SyncSet) error {
		syncCalled++
		syncDef = &def
		return nil
	}

	head, err := d.Repo.BranchHead(ctx)
	if err != nil {
		t.Fatal(err)
	}

	gitSync, _ := fluxsync.NewGitTagSyncProvider(d.Repo, syncTag, "", fluxsync.VerifySignaturesModeNone, d.GitConfig)
	syncState := &lastKnownSyncState{logger: d.Logger, state: gitSync}

	if err := d.Sync(ctx, time.Now().UTC(), head, syncState); err != nil {
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
		resource.IDs(gotResourceIDs).Sort()
		// Event should only have changed workload ids
		if !reflect.DeepEqual(gotResourceIDs, []resource.ID{resource.MustParseID("default:deployment/helloworld")}) {
			t.Errorf("Unexpected event workload ids: %#v, expected: %#v", gotResourceIDs, []resource.ID{resource.MustParseID("default:deployment/helloworld")})
		}
	}
	// It moves the tag
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := d.Repo.Refresh(ctx); err != nil {
		t.Errorf("pulling sync tag: %v", err)
	} else if revs, err := d.Repo.CommitsBetween(ctx, oldRevision, syncTag, false); err != nil {
		t.Errorf("finding revisions before sync tag: %v", err)
	} else if len(revs) <= 0 {
		t.Errorf("Should have moved sync tag forward")
	} else if revs[len(revs)-1].Revision != newRevision {
		t.Errorf("Should have moved sync tag to HEAD (%s), but was moved to: %s", newRevision, revs[len(revs)-1].Revision)
	}
}

func TestDoSync_WithErrors(t *testing.T) {
	d, cleanup := daemon(t)
	defer cleanup()

	expectedResourceIDs := resource.IDs{}
	for id := range testfiles.ResourceMap {
		expectedResourceIDs = append(expectedResourceIDs, id)
	}

	k8s.SyncFunc = func(def cluster.SyncSet) error {
		return nil
	}

	ctx := context.Background()
	head, err := d.Repo.BranchHead(ctx)
	if err != nil {
		t.Fatal(err)
	}

	syncTag := "sync"
	gitSync, _ := fluxsync.NewGitTagSyncProvider(d.Repo, syncTag, "", fluxsync.VerifySignaturesModeNone, d.GitConfig)
	syncState := &lastKnownSyncState{logger: d.Logger, state: gitSync}

	if err := d.Sync(ctx, time.Now().UTC(), head, syncState); err != nil {
		t.Error(err)
	}

	// Check 0 error stats
	checkSyncManifestsMetrics(t, len(expectedResourceIDs), 0)

	// Now add wrong manifest
	err = d.WithWorkingClone(ctx, func(checkout *git.Checkout) error {
		ctx, cancel := context.WithTimeout(ctx, 5000*time.Second)
		defer cancel()

		absolutePath := path.Join(checkout.Dir(), "error_manifest.yaml")
		if err := ioutil.WriteFile(absolutePath, []byte("Manifest that must produce errors"), 0600); err != nil {
			return err
		}
		commitAction := git.CommitAction{Author: "", Message: "test error commit"}
		err = checkout.CommitAndPush(ctx, commitAction, nil, true)
		if err != nil {
			return err
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	err = d.Repo.Refresh(ctx)
	if err != nil {
		t.Error(err)
	}

	if err := d.Sync(ctx, time.Now().UTC(), "HEAD", syncState); err != nil {
		// Check error not nil, manifest counters remain the same
		checkSyncManifestsMetrics(t, len(expectedResourceIDs), 0)
	} else {
		t.Error("Sync must fail because of invalid manifest")
	}

	// Fix manifest
	err = d.WithWorkingClone(ctx, func(checkout *git.Checkout) error {
		ctx, cancel := context.WithTimeout(ctx, 5000*time.Second)
		defer cancel()

		absolutePath := path.Join(checkout.Dir(), "error_manifest.yaml")
		if err := ioutil.WriteFile(absolutePath, []byte("# Just comment"), 0600); err != nil {
			return err
		}
		commitAction := git.CommitAction{Author: "", Message: "test fix commit"}
		err = checkout.CommitAndPush(ctx, commitAction, nil, true)
		if err != nil {
			return err
		}
		return err
	})

	if err != nil {
		t.Fatal(err)
	}

	err = d.Repo.Refresh(ctx)
	if err != nil {
		t.Error(err)
	}

	if err := d.Sync(ctx, time.Now().UTC(), "HEAD", syncState); err != nil {
		t.Error(err)
	}
	// Check 0 manifest error stats
	checkSyncManifestsMetrics(t, len(expectedResourceIDs), 0)

	// Emulate sync errors
	k8s.SyncFunc = func(def cluster.SyncSet) error {
		return cluster.SyncError{
			cluster.ResourceError{resource.MustParseID("mynamespace:deployment/depl1"), "src1", fmt.Errorf("Error1")},
			cluster.ResourceError{resource.MustParseID("mynamespace:deployment/depl2"), "src2", fmt.Errorf("Error2")},
		}
	}

	if err := d.Sync(ctx, time.Now().UTC(), "HEAD", syncState); err != nil {
		t.Error(err)
	}

	// Check 2 sync error in stats
	checkSyncManifestsMetrics(t, len(expectedResourceIDs)-2, 2)
}
