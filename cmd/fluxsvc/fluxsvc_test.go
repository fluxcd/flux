package main

import (
	"github.com/weaveworks/flux/server"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/db"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/history"
	historysql "github.com/weaveworks/flux/history/sql"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/http/client"
	httpserver "github.com/weaveworks/flux/http/server"
	"github.com/weaveworks/flux/instance"
	instancedb "github.com/weaveworks/flux/instance/sql"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"
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

	// Test Automate
	err := apiClient.Automate("", helloWorldSvc)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := instanceDB.GetConfig(flux.DefaultInstanceID)
	if !cfg.Services[helloWorldSvc].Automated {
		t.Fatal("Expected DB to record that it is automated. %#v", cfg.Services[helloWorldSvc])
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

	// Test Deautomate
	err := apiClient.Deautomate("", helloWorldSvc)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := instanceDB.GetConfig(flux.DefaultInstanceID)
	if cfg.Services[helloWorldSvc].Automated {
		t.Fatal("Expected DB to record that it is deautomated. %#v", cfg.Services[helloWorldSvc])
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

	// Test Lock
	err := apiClient.Lock("", helloWorldSvc)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := instanceDB.GetConfig(flux.DefaultInstanceID)
	if !cfg.Services[helloWorldSvc].Locked {
		t.Fatal("Expected DB to record that it is locked. %#v", cfg.Services[helloWorldSvc])
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

	// Test Unlock
	err := apiClient.Unlock("", helloWorldSvc)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := instanceDB.GetConfig(flux.DefaultInstanceID)
	if cfg.Services[helloWorldSvc].Locked {
		t.Fatal("Expected DB to record that it is unlocked. %#v", cfg.Services[helloWorldSvc])
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

	// Do something that will appear in the history
	apiClient.Lock("", helloWorldSvc)

	// Test History
	hist, err := apiClient.History("", helloWorldSvc)
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
	conf, err := apiClient.GetConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if conf.Git.Key != "******" {
		t.Fatalf("Expected hidden Key! %q but got %q", "******", conf.Git.Key)
	}
	if conf.Git.Branch != "exampleBranch" {
		t.Fatalf("Expected %q but got %q", "exampleBranch", conf.Git.Branch)
	}
}

func TestFluxsvc_GetConfigSingleSecret(t *testing.T) {
	setup()
	defer teardown()

	err := apiClient.SetConfig("", flux.UnsafeInstanceConfig{
		Git: flux.GitConfig{
			Branch: "dummy",
			Key:    "exampleKey",
		},
		Registry: flux.RegistryConfig{
			Auths: map[string]flux.Auth{
				"https://index.docker.io/v1/": flux.Auth{
					Auth: "dXNlcjpwYXNzd29yZA==",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// We only need to test that items are hidden here. All the standard
	// get/set/parse stuff is tested in config_test.go
	for _, v := range []struct {
		Key   string
		Value string
	}{
		{"git.key", "******"},                                           // Ensure git key is hidden
		{"registry.auths.'https://index.docker.io/v1/'", "user:******"}, // Get a map value
	} {
		resp, err := apiClient.GetConfigSingle("", v.Key, "")
		if err != nil {
			t.Fatal(v.Key, err)
		}
		if resp != v.Value {
			t.Fatalf("Expected %q but got %q", "exampleBranch", resp)
		}
	}
}

func TestFluxsvc_SetConfigSingle(t *testing.T) {
	setup()
	defer teardown()

	err := apiClient.SetConfigSingle("", flux.SingleConfigParams{
		Key:    "git.branch",
		Syntax: "yaml",
	}, "test")
	if err != nil {
		t.Fatal(err)
	}

	resp, err := apiClient.GetConfigSingle("", "git.branch", "")
	if err != nil {
		t.Fatal(err)
	}
	if resp != "test" {
		t.Fatal("Should have set config but got", resp)
	}
}

func TestFluxsvc_DeleteConfigSingle(t *testing.T) {
	setup()
	defer teardown()

	err := apiClient.SetConfig("", flux.UnsafeInstanceConfig{
		Git: flux.GitConfig{
			Branch: "dummy",
			Key:    "exampleKey",
		},
		Registry: flux.RegistryConfig{
			Auths: map[string]flux.Auth{
				"https://index.docker.io/v1/": flux.Auth{
					Auth: "dXNlcjpwYXNzd29yZA==",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = apiClient.DeleteConfigSingle("", flux.SingleConfigParams{
		Key:    "git.branch",
		Syntax: "yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := apiClient.GetConfigSingle("", "git.branch", "")
	if err != nil {
		t.Fatal(err)
	}
	if resp != "" {
		t.Fatal("Config should be blank (deleted)")
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
	conf, err := apiClient.GetConfig("")
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

	_, err := transport.NewDaemon(&http.Client{}, "", router, ts.URL, mockPlatform, log.NewNopLogger()) // For ping and for
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
