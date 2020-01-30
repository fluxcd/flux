package daemon

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/api/v10"
	"github.com/fluxcd/flux/pkg/api/v11"
	"github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/api/v9"
	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/cluster/kubernetes"
	"github.com/fluxcd/flux/pkg/cluster/mock"
	"github.com/fluxcd/flux/pkg/event"
	"github.com/fluxcd/flux/pkg/git"
	"github.com/fluxcd/flux/pkg/git/gittest"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/manifests"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/registry"
	registryMock "github.com/fluxcd/flux/pkg/registry/mock"
	"github.com/fluxcd/flux/pkg/resource"
	fluxsync "github.com/fluxcd/flux/pkg/sync"
	"github.com/fluxcd/flux/pkg/update"
)

const (
	// These have to match the values in cluster/kubernetes/testfiles/data.go
	wl                = "default:deployment/helloworld"
	container         = "greeter"
	ns                = "default"
	oldHelloImage     = "quay.io/weaveworks/helloworld:3" // older in time but newer version!
	newHelloImage     = "quay.io/weaveworks/helloworld:2"
	currentHelloImage = "quay.io/weaveworks/helloworld:master-a000001"

	anotherWl        = "another:deployment/service"
	anotherContainer = "it-doesn't-matter"
	anotherImage     = "another/service:latest"

	invalidNS   = "adsajkfldsa"
	testVersion = "test"
)

var testBytes = []byte(`{}`)

const timeout = 10 * time.Second

// When I ping, I should get a response
func TestDaemon_Ping(t *testing.T) {
	d, start, clean, _, _, _ := mockDaemon(t)
	start()
	defer clean()
	ctx := context.Background()
	if d.Ping(ctx) != nil {
		t.Fatal("Cluster did not return valid nil ping")
	}
}

// When I ask a version, I should get a version
func TestDaemon_Version(t *testing.T) {
	d, start, clean, _, _, _ := mockDaemon(t)
	start()
	defer clean()

	ctx := context.Background()
	v, err := d.Version(ctx)
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if v != testVersion {
		t.Fatalf("Expected %v but got %v", testVersion, v)
	}
}

// When I export it should export the current (mocked) k8s cluster
func TestDaemon_Export(t *testing.T) {
	d, start, clean, _, _, _ := mockDaemon(t)
	start()
	defer clean()

	ctx := context.Background()

	bytes, err := d.Export(ctx)
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if string(testBytes) != string(bytes) {
		t.Fatalf("Expected %v but got %v", string(testBytes), string(bytes))
	}
}

// When I call list workloads, it should list all the workloads
func TestDaemon_ListWorkloads(t *testing.T) {
	d, start, clean, _, _, _ := mockDaemon(t)
	start()
	defer clean()

	ctx := context.Background()

	// No namespace
	s, err := d.ListServices(ctx, "")
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if len(s) != 2 {
		t.Fatalf("Expected %v but got %v", 2, len(s))
	}

	// Just namespace
	s, err = d.ListServices(ctx, ns)
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if 1 != len(s) {
		t.Fatalf("Expected %v but got %v", 1, len(s))
	}

	// Invalid NS
	s, err = d.ListServices(ctx, invalidNS)
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if len(s) != 0 {
		t.Fatalf("Expected %v but got %v", 0, len(s))
	}
}

// When I call list workloads with options, it should list all the requested workloads
func TestDaemon_ListWorkloadsWithOptions(t *testing.T) {
	d, start, clean, _, _, _ := mockDaemon(t)
	start()
	defer clean()

	ctx := context.Background()

	t.Run("no filter", func(t *testing.T) {
		s, err := d.ListServicesWithOptions(ctx, v11.ListServicesOptions{})
		if err != nil {
			t.Fatalf("Error: %s", err.Error())
		}
		if len(s) != 2 {
			t.Fatalf("Expected %v but got %v", 2, len(s))
		}
	})
	t.Run("filter id", func(t *testing.T) {
		s, err := d.ListServicesWithOptions(ctx, v11.ListServicesOptions{
			Namespace: "",
			Services:  []resource.ID{resource.MustParseID(wl)}})
		if err != nil {
			t.Fatalf("Error: %s", err.Error())
		}
		if len(s) != 1 {
			t.Fatalf("Expected %v but got %v", 1, len(s))
		}
	})

	t.Run("filter id and namespace", func(t *testing.T) {
		_, err := d.ListServicesWithOptions(ctx, v11.ListServicesOptions{
			Namespace: "foo",
			Services:  []resource.ID{resource.MustParseID(wl)}})
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
	})

	t.Run("filter unsupported id kind", func(t *testing.T) {
		_, err := d.ListServicesWithOptions(ctx, v11.ListServicesOptions{
			Namespace: "foo",
			Services:  []resource.ID{resource.MustParseID("default:unsupportedkind/goodbyeworld")}})
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
	})
}

// When I call list images for a workload, it should return images
func TestDaemon_ListImagesWithOptions(t *testing.T) {
	d, start, clean, _, _, _ := mockDaemon(t)
	start()
	defer clean()

	ctx := context.Background()

	specAll := update.ResourceSpec(update.ResourceSpecAll)

	// Service 1
	svcID, err := resource.ParseID(wl)
	assert.NoError(t, err)
	currentImageRef, err := image.ParseRef(currentHelloImage)
	assert.NoError(t, err)
	newImageRef, err := image.ParseRef(newHelloImage)
	assert.NoError(t, err)
	oldImageRef, err := image.ParseRef(oldHelloImage)
	assert.NoError(t, err)

	// Service 2
	anotherSvcID, err := resource.ParseID(anotherWl)
	assert.NoError(t, err)
	anotherImageRef, err := image.ParseRef(anotherImage)
	assert.NoError(t, err)

	tests := []struct {
		name string
		opts v10.ListImagesOptions

		expectedImages    []v6.ImageStatus
		expectedNumImages int
		shouldError       bool
	}{
		{
			name: "All services",
			opts: v10.ListImagesOptions{Spec: specAll},
			expectedImages: []v6.ImageStatus{
				{
					ID: svcID,
					Containers: []v6.Container{
						{
							Name:           container,
							Current:        image.Info{ID: currentImageRef},
							LatestFiltered: image.Info{ID: newImageRef},
							Available: []image.Info{
								{ID: newImageRef},
								{ID: currentImageRef},
								{ID: oldImageRef},
							},
							AvailableImagesCount:    3,
							NewAvailableImagesCount: 1,
							FilteredImagesCount:     3,
							NewFilteredImagesCount:  1,
						},
					},
				},
				{
					ID: anotherSvcID,
					Containers: []v6.Container{
						{
							Name:           anotherContainer,
							Current:        image.Info{ID: anotherImageRef},
							LatestFiltered: image.Info{},
							Available: []image.Info{
								{ID: anotherImageRef},
							},
							AvailableImagesCount:    1,
							NewAvailableImagesCount: 0,
							FilteredImagesCount:     0, // Excludes latest
							NewFilteredImagesCount:  0,
						},
					},
				},
			},
			shouldError: false,
		},
		{
			name: "Specific service",
			opts: v10.ListImagesOptions{Spec: update.ResourceSpec(wl)},
			expectedImages: []v6.ImageStatus{
				{
					ID: svcID,
					Containers: []v6.Container{
						{
							Name:           container,
							Current:        image.Info{ID: currentImageRef},
							LatestFiltered: image.Info{ID: newImageRef},
							Available: []image.Info{
								{ID: newImageRef},
								{ID: currentImageRef},
								{ID: oldImageRef},
							},
							AvailableImagesCount:    3,
							NewAvailableImagesCount: 1,
							FilteredImagesCount:     3,
							NewFilteredImagesCount:  1,
						},
					},
				},
			},
			shouldError: false,
		},
		{
			name: "Override container field selection",
			opts: v10.ListImagesOptions{
				Spec:                    specAll,
				OverrideContainerFields: []string{"Name", "Current", "NewAvailableImagesCount"},
			},
			expectedImages: []v6.ImageStatus{
				{
					ID: svcID,
					Containers: []v6.Container{
						{
							Name:                    container,
							Current:                 image.Info{ID: currentImageRef},
							NewAvailableImagesCount: 1,
						},
					},
				},
				{
					ID: anotherSvcID,
					Containers: []v6.Container{
						{
							Name:                    anotherContainer,
							Current:                 image.Info{ID: anotherImageRef},
							NewAvailableImagesCount: 0,
						},
					},
				},
			},
			shouldError: false,
		},
		{
			name: "Override container field selection with invalid field",
			opts: v10.ListImagesOptions{
				Spec:                    specAll,
				OverrideContainerFields: []string{"InvalidField"},
			},
			expectedImages: nil,
			shouldError:    true,
		},
		{
			name: "Specific namespace",
			opts: v10.ListImagesOptions{
				Spec:      specAll,
				Namespace: ns,
			},
			expectedImages: []v6.ImageStatus{
				{
					ID: svcID,
					Containers: []v6.Container{
						{
							Name:           container,
							Current:        image.Info{ID: currentImageRef},
							LatestFiltered: image.Info{ID: newImageRef},
							Available: []image.Info{
								{ID: newImageRef},
								{ID: currentImageRef},
								{ID: oldImageRef},
							},
							AvailableImagesCount:    3,
							NewAvailableImagesCount: 1,
							FilteredImagesCount:     3,
							NewFilteredImagesCount:  1,
						},
					},
				},
			},
			shouldError: false,
		},
		{
			name: "Specific namespace and service",
			opts: v10.ListImagesOptions{
				Spec:      update.ResourceSpec(wl),
				Namespace: ns,
			},
			expectedImages: nil,
			shouldError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is, err := d.ListImagesWithOptions(ctx, tt.opts)
			assert.Equal(t, tt.shouldError, err != nil)

			// Clear CreatedAt fields for testing
			for ri, r := range is {
				for ci, c := range r.Containers {
					is[ri].Containers[ci].Current.CreatedAt = time.Time{}
					is[ri].Containers[ci].LatestFiltered.CreatedAt = time.Time{}
					for ai := range c.Available {
						is[ri].Containers[ci].Available[ai].CreatedAt = time.Time{}
					}
				}
			}

			assert.Equal(t, tt.expectedImages, is)
		})
	}
}

// When I call notify, it should cause a sync
func TestDaemon_NotifyChange(t *testing.T) {
	d, start, clean, mockK8s, events, _ := mockDaemon(t)

	w := newWait(t)
	ctx := context.Background()

	var syncCalled int
	var syncDef *cluster.SyncSet
	var syncMu sync.Mutex
	mockK8s.SyncFunc = func(def cluster.SyncSet) error {
		syncMu.Lock()
		syncCalled++
		syncDef = &def
		syncMu.Unlock()
		return nil
	}

	start()
	defer clean()

	d.NotifyChange(ctx, v9.Change{Kind: v9.GitChange, Source: v9.GitUpdate{}})
	w.Eventually(func() bool {
		syncMu.Lock()
		defer syncMu.Unlock()
		return syncCalled == 1
	}, "Waiting for sync called")

	// Check that sync was called
	syncMu.Lock()
	defer syncMu.Unlock()
	if syncCalled != 1 {
		t.Errorf("Sync was not called once, was called %d times", syncCalled)
	} else if syncDef == nil {
		t.Errorf("Sync was called with a nil syncDef")
	}

	// Check that history was written to
	w.Eventually(func() bool {
		es, _ := events.AllEvents(time.Time{}, -1, time.Time{})
		for _, e := range es {
			if e.Type == event.EventSync {
				return true
			}
		}
		return false
	}, "Waiting for new sync events")
}

// When I perform a release, it should add a job to update git to the queue
// When I ask about a Job, it should tell me about a job
// When I perform a release, it should update the git repo
func TestDaemon_Release(t *testing.T) {
	d, start, clean, _, _, _ := mockDaemon(t)
	start()
	defer clean()
	w := newWait(t)

	ctx := context.Background()

	// Perform a release
	id := updateImage(ctx, d, t)

	// Check that job is queued
	stat, err := d.JobStatus(ctx, id)
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	} else if stat.Err != "" {
		t.Fatal("Job status error should be empty")
	} else if stat.StatusString != job.StatusQueued {
		t.Fatalf("Expected %v but got %v", job.StatusQueued, stat.StatusString)
	}

	// Wait for job to succeed
	w.ForJobSucceeded(d, id)

	// Wait and check that the git manifest has been altered
	w.Eventually(func() bool {
		co, err := d.Repo.Clone(ctx, d.GitConfig)
		if err != nil {
			return false
		}
		defer co.Clean()
		// open a file
		dirs := co.AbsolutePaths()
		if file, err := os.Open(filepath.Join(dirs[0], "helloworld-deploy.yaml")); err == nil {

			// make sure it gets closed
			defer file.Close()

			// create a new scanner and read the file line by line
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				if strings.Contains(scanner.Text(), newHelloImage) {
					return true
				}
			}
		} else {
			t.Fatal(err)
		}
		// If we get here we haven't found the line we are looking for.
		return false
	}, "Waiting for new manifest")

}

// When I update a policy, I expect it to add to the queue
// When I update a policy, it should add an annotation to the manifest
func TestDaemon_PolicyUpdate(t *testing.T) {
	d, start, clean, _, _, _ := mockDaemon(t)
	start()
	defer clean()
	w := newWait(t)

	ctx := context.Background()
	// Push an update to a policy
	id := updatePolicy(ctx, t, d)

	// Wait for job to succeed
	w.ForJobSucceeded(d, id)

	// Wait and check for new annotation
	w.Eventually(func() bool {
		co, err := d.Repo.Clone(ctx, d.GitConfig)
		if err != nil {
			t.Error(err)
			return false
		}
		defer co.Clean()
		cm := manifests.NewRawFiles(co.Dir(), co.AbsolutePaths(), d.Manifests)
		m, err := cm.GetAllResourcesByID(context.TODO())
		if err != nil {
			t.Fatalf("Error: %s", err.Error())
		}
		return len(m[wl].Policies()) > 0
	}, "Waiting for new annotation")
}

// When I call sync status, it should return a commit showing the sync
// that is about to take place. Then it should return empty once it is
// complete
func TestDaemon_SyncStatus(t *testing.T) {
	d, start, clean, _, _, _ := mockDaemon(t)
	start()
	defer clean()
	w := newWait(t)

	ctx := context.Background()
	// Perform a release
	id := updateImage(ctx, d, t)

	// Get the commit id
	stat := w.ForJobSucceeded(d, id)

	// Note: I can't test for an expected number of commits > 0
	// because I can't control how fast the sync loop updates the cluster

	// Once sync'ed to the cluster, it should empty
	w.ForSyncStatus(d, stat.Result.Revision, 0)
}

// When I restart fluxd, there won't be any jobs in the cache
func TestDaemon_JobStatusWithNoCache(t *testing.T) {
	d, start, clean, _, _, restart := mockDaemon(t)
	start()
	defer clean()
	w := newWait(t)

	ctx := context.Background()
	// Perform update
	id := updatePolicy(ctx, t, d)

	// Make sure the job finishes first
	w.ForJobSucceeded(d, id)

	// Clear the cache like we've just restarted
	restart(func() {
		d.JobStatusCache = &job.StatusCache{Size: 100}
	})

	// Now check if we can get the job status from the commit
	w.ForJobSucceeded(d, id)
}

func TestDaemon_Automated(t *testing.T) {
	d, start, clean, k8s, _, _ := mockDaemon(t)
	defer clean()
	w := newWait(t)

	workload := cluster.Workload{
		ID: resource.MakeID(ns, "deployment", "helloworld"),
		Containers: cluster.ContainersOrExcuse{
			Containers: []resource.Container{
				{
					Name:  container,
					Image: mustParseImageRef(currentHelloImage),
				},
			},
		},
	}
	k8s.SomeWorkloadsFunc = func(ctx context.Context, ids []resource.ID) ([]cluster.Workload, error) {
		return []cluster.Workload{workload}, nil
	}
	start()

	// updates from helloworld:master-xxx to helloworld:2
	w.ForImageTag(t, d, wl, container, "2")
}

func TestDaemon_Automated_semver(t *testing.T) {
	d, start, clean, k8s, _, _ := mockDaemon(t)
	defer clean()
	w := newWait(t)

	resid := resource.MustParseID("default:deployment/semver")
	workload := cluster.Workload{
		ID: resid,
		Containers: cluster.ContainersOrExcuse{
			Containers: []resource.Container{
				{
					Name:  container,
					Image: mustParseImageRef(currentHelloImage),
				},
			},
		},
	}
	k8s.SomeWorkloadsFunc = func(ctx context.Context, ids []resource.ID) ([]cluster.Workload, error) {
		return []cluster.Workload{workload}, nil
	}
	start()

	// helloworld:3 is older than helloworld:2 but semver orders by version
	w.ForImageTag(t, d, resid.String(), container, "3")
}

func makeImageInfo(ref string, t time.Time) image.Info {
	return image.Info{ID: mustParseImageRef(ref), CreatedAt: t}
}

func mustParseImageRef(ref string) image.Ref {
	r, err := image.ParseRef(ref)
	if err != nil {
		panic(err)
	}
	return r
}

func mockDaemon(t *testing.T) (*Daemon, func(), func(), *mock.Mock, *mockEventWriter, func(func())) {
	logger := log.NewNopLogger()

	singleService := cluster.Workload{
		ID: resource.MustParseID(wl),
		Containers: cluster.ContainersOrExcuse{
			Containers: []resource.Container{
				{
					Name:  container,
					Image: mustParseImageRef(currentHelloImage),
				},
			},
		},
	}
	multiService := []cluster.Workload{
		singleService,
		{
			ID: resource.MakeID("another", "deployment", "service"),
			Containers: cluster.ContainersOrExcuse{
				Containers: []resource.Container{
					{
						Name:  anotherContainer,
						Image: mustParseImageRef(anotherImage),
					},
				},
			},
		},
	}

	repo, repoCleanup := gittest.Repo(t)

	syncTag := "flux-test"
	params := git.Config{
		Branch:    "master",
		UserName:  "example",
		UserEmail: "example@example.com",
		NotesRef:  "fluxtest",
	}

	var k8s *mock.Mock
	{
		k8s = &mock.Mock{}
		k8s.AllWorkloadsFunc = func(ctx context.Context, maybeNamespace string) ([]cluster.Workload, error) {
			if maybeNamespace == ns {
				return []cluster.Workload{
					singleService,
				}, nil
			} else if maybeNamespace == "" {
				return multiService, nil
			}
			return []cluster.Workload{}, nil
		}
		k8s.IsAllowedResourceFunc = func(resource.ID) bool { return true }
		k8s.ExportFunc = func(ctx context.Context) ([]byte, error) { return testBytes, nil }
		k8s.PingFunc = func() error { return nil }
		k8s.SomeWorkloadsFunc = func(ctx context.Context, ids []resource.ID) ([]cluster.Workload, error) {
			return []cluster.Workload{
				singleService,
			}, nil
		}
		k8s.SyncFunc = func(def cluster.SyncSet) error { return nil }
	}

	var imageRegistry registry.Registry
	{
		img0 := makeImageInfo(oldHelloImage, time.Now().Add(-1*time.Second))
		img1 := makeImageInfo(currentHelloImage, time.Now())
		img2 := makeImageInfo(newHelloImage, time.Now().Add(1*time.Second))
		img3 := makeImageInfo("another/service:latest", time.Now().Add(1*time.Second))
		imageRegistry = &registryMock.Registry{
			Images: []image.Info{
				img1,
				img2,
				img3,
				img0,
			},
		}
	}

	events := &mockEventWriter{}

	// Shutdown chan and waitgroups
	jshutdown := make(chan struct{})
	dshutdown := make(chan struct{})
	jwg := &sync.WaitGroup{}
	dwg := &sync.WaitGroup{}

	// Jobs queue (starts itself)
	jobs := job.NewQueue(jshutdown, jwg)

	manifests := kubernetes.NewManifests(kubernetes.ConstNamespacer("default"), log.NewLogfmtLogger(os.Stdout))

	gitSync, _ := fluxsync.NewGitTagSyncProvider(repo, syncTag, "", fluxsync.VerifySignaturesModeNone, params)

	// Finally, the daemon
	d := &Daemon{
		Repo:           repo,
		GitConfig:      params,
		Cluster:        k8s,
		Manifests:      manifests,
		Registry:       imageRegistry,
		V:              testVersion,
		Jobs:           jobs,
		JobStatusCache: &job.StatusCache{Size: 100},
		EventWriter:    events,
		Logger:         logger,
		LoopVars:       &LoopVars{SyncTimeout: timeout, GitTimeout: timeout, SyncState: gitSync, GitVerifySignaturesMode: fluxsync.VerifySignaturesModeNone},
	}

	start := func() {
		if err := repo.Ready(context.Background()); err != nil {
			t.Fatal(err)
		}

		dwg.Add(1)
		go d.Loop(dshutdown, dwg, logger)
	}

	stop := func() {
		// Close daemon first so any outstanding jobs are picked up by the queue, otherwise
		// calls to Queue#Enqueue() will block forever. Jobs may be enqueued if the daemon's
		// image polling picks up automated updates.
		close(dshutdown)
		dwg.Wait()
		close(jshutdown)
		jwg.Wait()
		repoCleanup()
	}

	restart := func(f func()) {
		close(dshutdown)
		dwg.Wait()

		f()

		dshutdown = make(chan struct{})
		start()
	}
	return d, start, stop, k8s, events, restart
}

type mockEventWriter struct {
	events []event.Event
	sync.Mutex
}

func (w *mockEventWriter) LogEvent(e event.Event) error {
	w.Lock()
	defer w.Unlock()
	w.events = append(w.events, e)
	return nil
}

func (w *mockEventWriter) AllEvents(_ time.Time, _ int64, _ time.Time) ([]event.Event, error) {
	w.Lock()
	defer w.Unlock()
	return w.events, nil
}

// DAEMON TEST HELPERS
type wait struct {
	t       *testing.T
	timeout time.Duration
}

func newWait(t *testing.T) wait {
	return wait{
		t:       t,
		timeout: timeout,
	}
}

const interval = 10 * time.Millisecond

func (w *wait) Eventually(f func() bool, msg string) {
	stop := time.Now().Add(w.timeout)
	for time.Now().Before(stop) {
		if f() {
			return
		}
		time.Sleep(interval)
	}
	w.t.Fatal(msg)
}

func (w *wait) ForJobSucceeded(d *Daemon, jobID job.ID) job.Status {
	var stat job.Status
	var err error

	ctx := context.Background()
	w.Eventually(func() bool {
		stat, err = d.JobStatus(ctx, jobID)
		if err != nil {
			return false
		}
		switch stat.StatusString {
		case job.StatusSucceeded:
			return true
		case job.StatusFailed:
			w.t.Fatal(stat.Err)
			return true
		default:
			return false
		}
	}, "Waiting for job to succeed")
	return stat
}

func (w *wait) ForSyncStatus(d *Daemon, rev string, expectedNumCommits int) []string {
	var revs []string
	var err error
	w.Eventually(func() bool {
		ctx := context.Background()
		revs, err = d.SyncStatus(ctx, rev)
		return err == nil && len(revs) == expectedNumCommits
	}, fmt.Sprintf("Waiting for sync status to have %d commits", expectedNumCommits))
	return revs
}

func (w *wait) ForImageTag(t *testing.T, d *Daemon, workload, container, tag string) {
	w.Eventually(func() bool {
		co, err := d.Repo.Clone(context.TODO(), d.GitConfig)
		if err != nil {
			return false
		}
		defer co.Clean()
		cm := manifests.NewRawFiles(co.Dir(), co.AbsolutePaths(), d.Manifests)
		resources, err := cm.GetAllResourcesByID(context.TODO())
		assert.NoError(t, err)

		workload, ok := resources[workload].(resource.Workload)
		assert.True(t, ok)
		for _, c := range workload.Containers() {
			if c.Name == container && c.Image.Tag == tag {
				return true
			}
		}
		return false
	}, fmt.Sprintf("Waiting for image tag: %q", tag))

}

func updateImage(ctx context.Context, d *Daemon, t *testing.T) job.ID {
	return updateManifest(ctx, t, d, update.Spec{
		Type: update.Images,
		Spec: update.ReleaseImageSpec{
			Kind:         update.ReleaseKindExecute,
			ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
			ImageSpec:    newHelloImage,
		},
	})
}

func updatePolicy(ctx context.Context, t *testing.T, d *Daemon) job.ID {
	return updateManifest(ctx, t, d, update.Spec{
		Type: update.Policy,
		Spec: resource.PolicyUpdates{
			resource.MustParseID("default:deployment/helloworld"): {
				Add: policy.Set{
					policy.Locked: "true",
				},
			},
		},
	})
}

func updateManifest(ctx context.Context, t *testing.T, d *Daemon, spec update.Spec) job.ID {
	id, err := d.UpdateManifests(ctx, spec)
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if id == "" {
		t.Fatal("id should not be empty")
	}
	return id
}
