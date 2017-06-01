package daemon

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/cluster/kubernetes"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/update"
)

const (
	svc               = "default/helloworld"
	ns                = "default"
	invalidNS         = "adsajkfldsa"
	testVersion       = "test"
	newHelloImage     = "quay.io/weaveworks/helloworld:2"
	currentHelloImage = "quay.io/weaveworks/helloworld:master-a000001"
)

var (
	testBytes = []byte(`{}`)
	timeout   = 5 * time.Second
)

// When I ping, I should get a response
func TestDaemon_Ping(t *testing.T) {
	d, clean, _, _ := mockDaemon(t)
	defer clean()
	if d.Ping() != nil {
		t.Fatal("Cluster did not return valid nil ping")
	}
}

// When I ask a version, I should get a version
func TestDaemon_Version(t *testing.T) {
	d, clean, _, _ := mockDaemon(t)
	defer clean()

	v, err := d.Version()
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if v != testVersion {
		t.Fatalf("Expected %v but got %v", testVersion, v)
	}
}

// When I export it should export the current (mocked) k8s cluster
func TestDaemon_Export(t *testing.T) {
	d, clean, _, _ := mockDaemon(t)
	defer clean()

	bytes, err := d.Export()
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if string(testBytes) != string(bytes) {
		t.Fatalf("Expected %v but got %v", string(testBytes), string(bytes))
	}
}

// When I call list services, it should list all the services
func TestDaemon_ListServices(t *testing.T) {
	d, clean, _, _ := mockDaemon(t)
	defer clean()

	// No namespace
	s, err := d.ListServices("")
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if len(s) != 2 {
		t.Fatalf("Expected %v but got %v", 2, len(s))
	}

	// Just namespace
	s, err = d.ListServices(ns)
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if 1 != len(s) {
		t.Fatalf("Expected %v but got %v", 1, len(s))
	}

	// Invalid NS
	s, err = d.ListServices(invalidNS)
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if len(s) != 0 {
		t.Fatalf("Expected %v but got %v", 0, len(s))
	}
}

// When I call list images for a service, it should return images
func TestDaemon_ListImages(t *testing.T) {
	d, clean, _, _ := mockDaemon(t)
	defer clean()

	// List all images for services
	ss := update.ServiceSpec(update.ServiceSpecAll)
	is, err := d.ListImages(ss)
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	ids := imageIDs(is)
	if 3 != len(ids) {
		t.Fatalf("Expected %v but got %v", 3, len(ids))
	}

	// List images for specific service
	ss = update.ServiceSpec(svc)
	is, err = d.ListImages(ss)
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	ids = imageIDs(is)
	if 2 != len(ids) {
		t.Fatalf("Expected %v but got %v", 2, len(ids))
	}
}

// When I call sync notify, it should cause a sync
func TestDaemon_SyncNotify(t *testing.T) {
	d, clean, mockK8s, events := mockDaemon(t)
	defer clean()
	w := newWait(t)

	var syncCalled int
	var syncDef *cluster.SyncDef
	var syncMu sync.Mutex
	mockK8s.SyncFunc = func(def cluster.SyncDef) error {
		syncMu.Lock()
		syncCalled++
		syncDef = &def
		syncMu.Unlock()
		return nil
	}

	d.SyncNotify()
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
	} else if len(syncDef.Actions) != len(testfiles.Files) {
		t.Errorf("Sync was not called with the %d actions, was called with: %d", len(testfiles.Files), len(syncDef.Actions))
	}

	// Check that history was written to
	var e []history.Event
	w.Eventually(func() bool {
		e, _ = events.AllEvents(time.Time{}, -1)
		return len(e) > 0
	}, "Waiting for new events")
	if 1 != len(e) {
		t.Fatal("Expected one log event from the sync, but got", len(e))
	} else if history.EventSync != e[0].Type {
		t.Fatalf("Expected event with type %s but got %s", history.EventSync, e[0].Type)
	}
}

// When I perform a release, it should add a job to update git to the queue
// When I ask about a Job, it should tell me about a job
// When I perform a release, it should update the git repo
func TestDaemon_Release(t *testing.T) {
	d, clean, _, _ := mockDaemon(t)
	defer clean()
	w := newWait(t)

	// Perform a release
	id := updateImage(d, t)

	// Check that job is queued
	stat, err := d.JobStatus(id)
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
		// open a file
		if file, err := os.Open(filepath.Join(d.Checkout.ManifestDir(), "helloworld-deploy.yaml")); err == nil {

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
	d, clean, _, _ := mockDaemon(t)
	defer clean()
	w := newWait(t)

	// Push an update to a policy
	id := updatePolicy(t, d)

	// Wait for job to succeed
	w.ForJobSucceeded(d, id)

	// Wait and check for new annotation
	w.Eventually(func() bool {
		d.Checkout.Lock()
		m, err := d.Manifests.LoadManifests(d.Checkout.ManifestDir())
		if err != nil {
			t.Fatalf("Error: %s", err.Error())
		}
		d.Checkout.Unlock()
		return m["Deployment "+svc].Annotations() != nil
	}, "Waiting for new annotation")
}

// When I call sync status, it should return a commit showing the sync
// that is about to take place. Then it should return empty once it is
// complete
func TestDaemon_SyncStatus(t *testing.T) {
	d, clean, _, _ := mockDaemon(t)
	defer clean()
	w := newWait(t)

	// Perform a release
	id := updateImage(d, t)

	// Get the commit id
	stat := w.ForJobSucceeded(d, id)

	// Note: I can't test for an expected number of commits > 0
	// because I can't control how fast the sync loop updates the cluster

	// Once sync'ed to the cluster, it should empty
	w.ForSyncStatus(d, stat.Result.Revision, 0)
}

// When I restart fluxd, there won't be any jobs in the cache
func TestDaemon_JobStatusWithNoCache(t *testing.T) {
	d, clean, _, _ := mockDaemon(t)
	defer clean()
	w := newWait(t)

	// Perform update
	id := updatePolicy(t, d)

	// Make sure the job finishes first
	w.ForJobSucceeded(d, id)

	// Clear the cache like we've just restarted
	d.JobStatusCache = &job.StatusCache{Size: 100}

	// Now check if we can get the job status from the commit
	w.ForJobSucceeded(d, id)
}

func mockDaemon(t *testing.T) (*Daemon, func(), *cluster.Mock, history.EventReadWriter) {
	logger := log.NewLogfmtLogger(os.Stdout)

	singleService := cluster.Service{
		ID: flux.ServiceID(svc),
		Containers: cluster.ContainersOrExcuse{
			Containers: []cluster.Container{
				{
					Name:  svc,
					Image: currentHelloImage,
				},
			},
		},
	}
	multiService := []cluster.Service{
		singleService,
		cluster.Service{
			ID: flux.MakeServiceID("another", "service"),
			Containers: cluster.ContainersOrExcuse{
				Containers: []cluster.Container{
					{
						Name:  "it doesn't matter",
						Image: "another/service:latest",
					},
				},
			},
		},
	}

	repo, repoCleanup := gittest.Repo(t)
	params := git.Config{
		UserName:  "example",
		UserEmail: "example@example.com",
		SyncTag:   "flux-test",
		NotesRef:  "fluxtest",
	}
	checkout, err := repo.Clone(params)
	if err != nil {
		t.Fatal(err)
	}

	var k8s *cluster.Mock
	{
		k8s = &cluster.Mock{}
		k8s.AllServicesFunc = func(maybeNamespace string) ([]cluster.Service, error) {
			if maybeNamespace == ns {
				return []cluster.Service{
					singleService,
				}, nil
			} else if maybeNamespace == "" {
				return multiService, nil
			}
			return []cluster.Service{}, nil
		}
		k8s.ExportFunc = func() ([]byte, error) { return testBytes, nil }
		k8s.FindDefinedServicesFunc = (&kubernetes.Manifests{}).FindDefinedServices
		k8s.LoadManifestsFunc = kresource.Load
		k8s.ParseManifestsFunc = func(allDefs []byte) (map[string]resource.Resource, error) {
			return kresource.ParseMultidoc(allDefs, "test")
		}
		k8s.PingFunc = func() error { return nil }
		k8s.ServicesWithPolicyFunc = (&kubernetes.Manifests{}).ServicesWithPolicy
		k8s.SomeServicesFunc = func([]flux.ServiceID) ([]cluster.Service, error) {
			return []cluster.Service{
				singleService,
			}, nil
		}
		k8s.SyncFunc = func(def cluster.SyncDef) error { return nil }
		k8s.UpdatePoliciesFunc = (&kubernetes.Manifests{}).UpdatePolicies
		k8s.UpdateDefinitionFunc = (&kubernetes.Manifests{}).UpdateDefinition
	}

	var imageRegistry registry.Registry
	{
		img1, _ := flux.ParseImage(currentHelloImage, time.Now())
		img2, _ := flux.ParseImage(newHelloImage, time.Now().Add(1*time.Second))
		img3, _ := flux.ParseImage("another/service:latest", time.Now().Add(1*time.Second))
		imageRegistry = registry.NewMockRegistry([]flux.Image{
			img1,
			img2,
			img3,
		}, nil)
	}

	events := history.NewMock()

	// Shutdown chans and waitgroups
	shutdown := make(chan struct{})
	wg := &sync.WaitGroup{}

	// Jobs queue
	jobs := job.NewQueue(shutdown, wg)

	// Finally, the daemon
	d := &Daemon{
		Checkout:       checkout,
		Cluster:        k8s,
		Manifests:      &kubernetes.Manifests{},
		Registry:       imageRegistry,
		V:              testVersion,
		Jobs:           jobs,
		JobStatusCache: &job.StatusCache{Size: 100},
		EventWriter:    events,
	}

	wg.Add(1)
	go d.Loop(shutdown, wg, logger)

	return d, func() {
		// Close daemon first so we don't get errors if the queue closes before the daemon
		close(shutdown)
		wg.Wait() // Wait for it to close, it might take a while
		repoCleanup()
	}, k8s, events
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
	w.Eventually(func() bool {
		stat, err = d.JobStatus(jobID)
		return err == nil && stat.StatusString == job.StatusSucceeded
	}, "Waiting for job to succeed")
	return stat
}

func (w *wait) ForSyncStatus(d *Daemon, rev string, expectedNumCommits int) []string {
	var revs []string
	var err error
	w.Eventually(func() bool {
		revs, err = d.SyncStatus(rev)
		return err == nil && len(revs) == expectedNumCommits
	}, fmt.Sprintf("Waiting for sync status to have %d commits", expectedNumCommits))
	return revs
}

func imageIDs(status []flux.ImageStatus) []flux.Image {
	var availableImgs []flux.Image
	for _, i := range status {
		for _, c := range i.Containers {
			availableImgs = append(availableImgs, c.Available...)
		}
	}
	return availableImgs
}

func updateImage(d *Daemon, t *testing.T) job.ID {
	return updateManifest(t, d, update.Spec{
		Type: update.Images,
		Spec: update.ReleaseSpec{
			ImageSpec: newHelloImage,
		},
	})
}

func updatePolicy(t *testing.T, d *Daemon) job.ID {
	return updateManifest(t, d, update.Spec{
		Type: update.Policy,
		Spec: policy.Updates{
			"default/helloworld": {
				Add: []policy.Policy{
					policy.Locked,
				},
			},
		},
	})
}
func updateManifest(t *testing.T, d *Daemon, spec update.Spec) job.ID {
	id, err := d.UpdateManifests(spec)
	if err != nil {
		t.Fatalf("Error: %s", err.Error())
	}
	if id == "" {
		t.Fatal("id should not be empty")
	}
	return id
}
