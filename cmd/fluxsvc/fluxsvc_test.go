package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/db"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/history"
	historysql "github.com/weaveworks/flux/history/sql"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/http/client"
	httpserver "github.com/weaveworks/flux/http/server"
	"github.com/weaveworks/flux/instance"
	instancedb "github.com/weaveworks/flux/instance/sql"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/platform/kubernetes"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/server"
)

var (
	// server is a test HTTP server used to provide mock API responses.
	ts *httptest.Server

	// This is a connection to the jobs DB. Use this for validation.
	jobStore jobs.JobStore

	// Stores information about service configuration (e.g. automation)
	instanceDB instance.DB

	// Mux router
	router *mux.Router

	// Mocked out platform. Global for use in Register test.
	mockPlatform platform.Platform

	// API Client
	apiClient api.ClientService
)

const (
	helloWorldSvc = "default/helloworld"
	ver           = "123"
)

func setup() {
	git.KeySize = 128
	databaseSource := "file://fluxy.db"
	databaseMigrationsDir, _ := filepath.Abs("../../db/migrations")
	var dbDriver string
	{
		db.Migrate(databaseSource, databaseMigrationsDir)
		dbDriver = db.DriverForScheme("file")
	}

	// Job store.
	s, _ := jobs.NewDatabaseStore(dbDriver, databaseSource, time.Hour)
	jobStore = jobs.InstrumentedJobStore(s)

	// Message bus
	messageBus := platform.NewStandaloneMessageBus(platform.BusMetricsImpl)

	mockPlatform = &platform.MockPlatform{
		AllServicesAnswer: []platform.Service{
			platform.Service{
				ID:       flux.ServiceID(helloWorldSvc),
				IP:       "10.32.1.45",
				Metadata: map[string]string{},
				Status:   "ok",
				Containers: platform.ContainersOrExcuse{
					Containers: []platform.Container{
						platform.Container{
							Name:  "helloworld",
							Image: "alpine:latest",
						},
					},
				},
			},
			platform.Service{},
		},
		SomeServicesAnswer: []platform.Service{
			platform.Service{
				ID:       flux.ServiceID(helloWorldSvc),
				IP:       "10.32.1.45",
				Metadata: map[string]string{},
				Status:   "ok",
				Containers: platform.ContainersOrExcuse{
					Containers: []platform.Container{
						platform.Container{
							Name:  "helloworld",
							Image: "alpine:latest",
						},
					},
				},
			},
		},
	}
	done := make(chan error)
	messageBus.Subscribe(flux.DefaultInstanceID, mockPlatform, done) // For ListService

	// History
	hDb, _ := historysql.NewSQL(dbDriver, databaseSource)
	historyDB := history.InstrumentedDB(hDb)

	// Instancer
	db, _ := instancedb.New(dbDriver, databaseSource)
	instanceDB = instance.InstrumentedDB(db)

	var instancer instance.Instancer
	{
		// Instancer, for the instancing of operations
		instancer = &instance.MultitenantInstancer{
			DB:        instanceDB,
			Connecter: messageBus,
			Logger:    log.NewNopLogger(),
			History:   historyDB,
		}
	}

	// Server
	apiServer := server.New(ver, instancer, instanceDB, messageBus, jobStore, log.NewNopLogger())
	router = transport.NewRouter()
	handler := httpserver.NewHandler(apiServer, router, log.NewNopLogger())
	ts = httptest.NewServer(handler)
	apiClient = client.New(http.DefaultClient, router, ts.URL, "")
}

func teardown() {
	ts.Close()
}

func TestFluxsvc_ListServices(t *testing.T) {
	setup()
	defer teardown()

	// Set up the git repo
	repo, cleanup := gittest.Repo(t)
	defer cleanup()
	err := apiClient.SetConfig("", flux.UnsafeInstanceConfig{
		Git: flux.GitConfig{
			URL:    repo.URL,
			Branch: repo.Branch,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test ListServices
	svcs, err := apiClient.ListServices("", "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 2 {
		t.Fatal("Expected there to be two services")
	}
	if svcs[0].ID != helloWorldSvc && svcs[1].ID != helloWorldSvc {
		t.Fatalf("Expected one of the services to be %q", helloWorldSvc)
	}

	// Test no namespace error
	u, _ := transport.MakeURL(ts.URL, router, "ListServices")
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request should not exist: %q", resp.Status)
	}
}

// Note that this test will reach out to docker hub to check the images
// associated with alpine
func TestFluxsvc_ListImages(t *testing.T) {
	setup()
	defer teardown()

	// Test ListImages
	imgs, err := apiClient.ListImages("", flux.ServiceSpecAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Fatal("Expected there two sets of images")
	}
	if len(imgs[0].Containers) == 0 && len(imgs[1].Containers) == 0 {
		t.Fatal("Should have been lots of containers")
	}

	// Test ListImages for specific service
	imgs, err = apiClient.ListImages("", helloWorldSvc)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 {
		t.Fatal("Expected there two sets of images")
	}
	if len(imgs[0].Containers) == 0 {
		t.Fatal("Should have been lots of containers")
	}

	// Test no service error
	u, _ := transport.MakeURL(ts.URL, router, "ListImages")
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request should not exist: %q", resp.Status)
	}
}

func TestFluxsvc_Release(t *testing.T) {
	setup()
	defer teardown()

	// Test PostRelease
	r, err := apiClient.PostRelease("", jobs.ReleaseJobParams{
		ReleaseSpec: flux.ReleaseSpec{
			ImageSpec:    "alpine:latest",
			Kind:         "execute",
			ServiceSpecs: []flux.ServiceSpec{helloWorldSvc},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	j, err := jobStore.GetJob(flux.DefaultInstanceID, r)
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != jobs.StatusQueued {
		t.Fatalf("Job should have been queued but was %q", j.Status)
	}
	if j.Method != jobs.ReleaseJob {
		t.Fatalf("Job should have been of type %q", jobs.ReleaseJob)
	}

	// Test GetRelease
	res, err := apiClient.GetRelease("", r)
	if err != nil {
		t.Fatal(err)
	}
	if res.Method != jobs.ReleaseJob {
		t.Fatalf("Job should have been of type %q", jobs.ReleaseJob)
	}

	// Test GetRelease doesn't exist
	_, err = apiClient.GetRelease("", "does-not-exist")
	if err == nil {
		t.Fatal("Should have errored due to not existing")
	}

	// Test PostRelease without parameters
	u, _ := transport.MakeURL(ts.URL, router, "PostRelease", "service", "default/service")
	resp, err := http.Post(u.String(), "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Path should 404: %q", resp.Status)
	}
}

func TestFluxsvc_Automate(t *testing.T) {
	setup()
	defer teardown()

	// Set up the git repo
	repo, cleanup := gittest.Repo(t)
	defer cleanup()
	err := apiClient.SetConfig("", flux.UnsafeInstanceConfig{
		Git: flux.GitConfig{
			URL:    repo.URL,
			Branch: repo.Branch,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test Automate
	err = apiClient.UpdatePolicies("", policy.Updates{
		helloWorldSvc: policy.Update{Add: []policy.Policy{policy.Automated}},
	})
	if err != nil {
		t.Fatal(err)
	}

	path, err := repo.Clone()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)
	automated, err := kubernetes.ServicesWithPolicy(path, policy.Automated)
	if err != nil {
		t.Fatal(err)
	}
	if !automated.Contains(helloWorldSvc) {
		t.Fatal("Expected repo to record that it is automated. Automated services: %v", automated)
	}

	// Test no service error
	u, _ := transport.MakeURL(ts.URL, router, "Automate")
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request should not exist: %q", resp.Status)
	}
}

func TestFluxsvc_Deautomate(t *testing.T) {
	setup()
	defer teardown()

	// Set up the git repo
	repo, cleanup := gittest.Repo(t)
	defer cleanup()
	err := apiClient.SetConfig("", flux.UnsafeInstanceConfig{
		Git: flux.GitConfig{
			URL:    repo.URL,
			Branch: repo.Branch,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test Deautomate
	err = apiClient.UpdatePolicies("", policy.Updates{
		helloWorldSvc: policy.Update{Remove: []policy.Policy{policy.Automated}},
	})
	if err != nil {
		t.Fatal(err)
	}

	path, err := repo.Clone()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)
	automated, err := kubernetes.ServicesWithPolicy(path, policy.Automated)
	if err != nil {
		t.Fatal(err)
	}
	if automated.Contains(helloWorldSvc) {
		t.Fatal("Expected repo to record that it is deautomated. Automated services: %v", automated)
	}

	// Test no service error
	u, _ := transport.MakeURL(ts.URL, router, "Deautomate")
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request should not exist: %q", resp.Status)
	}
}

func TestFluxsvc_Lock(t *testing.T) {
	setup()
	defer teardown()

	// Set up the git repo
	repo, cleanup := gittest.Repo(t)
	defer cleanup()
	err := apiClient.SetConfig("", flux.UnsafeInstanceConfig{
		Git: flux.GitConfig{
			URL:    repo.URL,
			Branch: repo.Branch,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test Lock
	err = apiClient.UpdatePolicies("", policy.Updates{
		helloWorldSvc: policy.Update{Add: []policy.Policy{policy.Locked}},
	})
	if err != nil {
		t.Fatal(err)
	}

	path, err := repo.Clone()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)
	locked, err := kubernetes.ServicesWithPolicy(path, policy.Locked)
	if err != nil {
		t.Fatal(err)
	}
	if !locked.Contains(helloWorldSvc) {
		t.Fatal("Expected repo to record that it is locked. Locked services: %v", locked)
	}

	// Test no service error
	u, _ := transport.MakeURL(ts.URL, router, "Lock")
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request should not exist: %q", resp.Status)
	}
}

func TestFluxsvc_Unlock(t *testing.T) {
	setup()
	defer teardown()

	// Set up the git repo
	repo, cleanup := gittest.Repo(t)
	defer cleanup()
	err := apiClient.SetConfig("", flux.UnsafeInstanceConfig{
		Git: flux.GitConfig{
			URL:    repo.URL,
			Branch: repo.Branch,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test Unlock
	err = apiClient.UpdatePolicies("", policy.Updates{
		helloWorldSvc: policy.Update{Remove: []policy.Policy{policy.Locked}},
	})
	if err != nil {
		t.Fatal(err)
	}

	path, err := repo.Clone()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)
	locked, err := kubernetes.ServicesWithPolicy(path, policy.Locked)
	if err != nil {
		t.Fatal(err)
	}
	if locked.Contains(helloWorldSvc) {
		t.Fatal("Expected repo to record that it is unlocked. Locked services: %v", locked)
	}

	// Test no service error
	u, _ := transport.MakeURL(ts.URL, router, "Unlock")
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request should not exist: %q", resp.Status)
	}
}

func TestFluxsvc_History(t *testing.T) {
	setup()
	defer teardown()

	// Set up the git repo
	repo, cleanup := gittest.Repo(t)
	defer cleanup()
	err := apiClient.SetConfig("", flux.UnsafeInstanceConfig{
		Git: flux.GitConfig{
			URL:    repo.URL,
			Branch: repo.Branch,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Do something that will appear in the history
	err = apiClient.UpdatePolicies("", policy.Updates{
		helloWorldSvc: policy.Update{Add: []policy.Policy{policy.Locked}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test History
	hist, err := apiClient.History("", helloWorldSvc, time.Now().UTC(), -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) == 0 {
		t.Fatal("History should be longer than this: ", hist)
	}
	var hasLock bool
	for _, v := range hist {
		if strings.Contains(v.Data, "locked") {
			hasLock = true
			break
		}
	}
	if !hasLock {
		t.Fatal("History hasn't recorded a lock", hist)
	}

	// Test no service error
	u, _ := transport.MakeURL(ts.URL, router, "History")
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request should not exist: %q", resp.Status)
	}
}

func TestFluxsvc_Status(t *testing.T) {
	setup()
	defer teardown()

	// Test Status
	status, err := apiClient.Status("")
	if err != nil {
		t.Fatal(err)
	}
	if status.Fluxsvc.Version != ver {
		t.Fatal("Expected %q, got %q", ver, status.Fluxsvc.Version)
	}
}

func TestFluxsvc_Config(t *testing.T) {
	setup()
	defer teardown()

	// Test that config is written
	err := apiClient.SetConfig("", flux.UnsafeInstanceConfig{
		Git: flux.GitConfig{
			Key:    "exampleKey",
			Branch: "exampleBranch",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	conf, err := apiClient.GetConfig("", "")
	if err != nil {
		t.Fatal(err)
	}
	if conf.Git.Key != "******" {
		t.Fatalf("Expected hidden Key! %q but got %q", "******", conf.Git.Key)
	}
	if conf.Git.Branch != "exampleBranch" {
		t.Fatalf("Expected %q but got %q", "exampleBranch", conf.Git.Key)
	}
}

func TestFluxsvc_DeployKeys(t *testing.T) {
	setup()
	defer teardown()

	// Ensure empty key
	err := apiClient.SetConfig("", flux.UnsafeInstanceConfig{
		Git: flux.GitConfig{
			Key: "",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Generate key
	err = apiClient.GenerateDeployKey("")
	if err != nil {
		t.Fatal(err)
	}

	// Get new key
	conf, err := apiClient.GetConfig("", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(conf.Git.Key, "ssh-rsa") {
		t.Fatalf("Expected proper ssh key but got %q", conf.Git.Key)
	}
}

func TestFluxsvc_Ping(t *testing.T) {
	setup()
	defer teardown()

	// Test Ping
	u, _ := transport.MakeURL(ts.URL, router, "IsConnected")
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		io.Copy(os.Stdout, resp.Body)
		t.Fatal("Request should have been ok but got %q", resp.Status)
	}
}

func TestFluxsvc_Register(t *testing.T) {
	setup()
	defer teardown()

	_, err := transport.NewDaemon(&http.Client{}, "fluxd/test", "", router, ts.URL, mockPlatform, log.NewNopLogger()) // For ping and for
	if err != nil {
		t.Fatal(err)
	}

	// Test Ping to make sure daemon has registered.
	u, _ := transport.MakeURL(ts.URL, router, "IsConnected")
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		io.Copy(os.Stdout, resp.Body)
		t.Fatal("Request should have been ok but got %q", resp.Status)
	}
}
