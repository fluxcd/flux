package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/db"
	"github.com/weaveworks/flux/guid"
	"github.com/weaveworks/flux/history"
	historysql "github.com/weaveworks/flux/history/sql"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/http/client"
	httpdaemon "github.com/weaveworks/flux/http/daemon"
	httpserver "github.com/weaveworks/flux/http/server"
	"github.com/weaveworks/flux/instance"
	instancedb "github.com/weaveworks/flux/instance/sql"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/server"
	"github.com/weaveworks/flux/update"
	"io/ioutil"
)

var (
	// server is a test HTTP server used to provide mock API responses.
	ts *httptest.Server

	// Stores information about service configuration (e.g. automation)
	instanceDB instance.DB

	// Mux router
	router *mux.Router

	// Mocked out remote platform.
	mockPlatform *remote.MockPlatform

	// API Client
	apiClient api.ClientService
)

const (
	helloWorldSvc = "default/helloworld"
	ver           = "123"
)

func setup() {
	databaseSource := "file://fluxy.db"
	databaseMigrationsDir, _ := filepath.Abs("../../db/migrations")
	var dbDriver string
	{
		db.Migrate(databaseSource, databaseMigrationsDir)
		dbDriver = db.DriverForScheme("file")
	}

	// Message bus
	messageBus := remote.NewStandaloneMessageBus(remote.BusMetricsImpl)

	imageID, _ := flux.ParseImageID("quay.io/weaveworks/helloworld:v1")
	mockPlatform = &remote.MockPlatform{
		ListServicesAnswer: []flux.ServiceStatus{
			flux.ServiceStatus{
				ID:     flux.ServiceID(helloWorldSvc),
				Status: "ok",
				Containers: []flux.Container{
					flux.Container{
						Name: "helloworld",
						Current: flux.Image{
							ID: imageID,
						},
					},
				},
			},
			flux.ServiceStatus{},
		},
		ListImagesAnswer: []flux.ImageStatus{
			flux.ImageStatus{
				ID: flux.ServiceID(helloWorldSvc),
				Containers: []flux.Container{
					flux.Container{
						Name: "helloworld",
						Current: flux.Image{
							ID: imageID,
						},
					},
				},
			},
			flux.ImageStatus{
				ID: flux.ServiceID("a/another"),
				Containers: []flux.Container{
					flux.Container{
						Name: "helloworld",
						Current: flux.Image{
							ID: imageID,
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
	apiServer := server.New(ver, instancer, instanceDB, messageBus, log.NewNopLogger())
	router = transport.NewServiceRouter()
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
		t.Error(err)
	}
	if len(svcs) != 2 {
		t.Error("Expected there to be two services")
	}
	if svcs[0].ID != helloWorldSvc && svcs[1].ID != helloWorldSvc {
		t.Errorf("Expected one of the services to be %q", helloWorldSvc)
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
	imgs, err := apiClient.ListImages("", update.ServiceSpecAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Error("Expected there two sets of images")
	}
	if len(imgs[0].Containers) == 0 && len(imgs[1].Containers) == 0 {
		t.Error("Should have been lots of containers")
	}

	// Test ListImages for specific service
	imgs, err = apiClient.ListImages("", helloWorldSvc)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Error("Expected two sets of images")
	}
	if len(imgs[0].Containers) == 0 {
		t.Error("Expected >1 containers")
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

	mockPlatform.UpdateManifestsAnswer = job.ID(guid.New())
	mockPlatform.JobStatusAnswer = job.Status{
		StatusString: job.StatusQueued,
	}

	// Test UpdateImages
	r, err := apiClient.UpdateImages("", update.ReleaseSpec{
		ImageSpec:    "alpine:latest",
		Kind:         "execute",
		ServiceSpecs: []update.ServiceSpec{helloWorldSvc},
	}, update.Cause{})
	if err != nil {
		t.Fatal(err)
	}
	if r != mockPlatform.UpdateManifestsAnswer {
		t.Error("%q != %q", r, mockPlatform.UpdateManifestsAnswer)
	}

	// Test GetRelease
	res, err := apiClient.JobStatus("", r)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusString != job.StatusQueued {
		t.Error("Unexpected job status: " + res.StatusString)
	}

	// Test JobStatus without parameters
	u, _ := transport.MakeURL(ts.URL, router, "UpdateImages", "service", "default/service")
	resp, err := http.Post(u.String(), "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Path should 404: %q", resp.Status)
	}
}

func TestFluxsvc_History(t *testing.T) {
	setup()
	defer teardown()

	// Post an event to the history. We have to cheat a bit here and
	// make a blind cast, because we want LogEvent, which the client
	// implements for convenient use in the daemon, without
	// implementing the other parts of api.DaemonService.
	eventLogger, ok := apiClient.(interface {
		LogEvent(flux.InstanceID, history.Event) error
	})
	if !ok {
		t.Fatal("API client does not implement LogEvent (maybe that method has moved)")
	}
	err := eventLogger.LogEvent("", history.Event{
		Type: history.EventLock,
		ServiceIDs: []flux.ServiceID{
			helloWorldSvc,
		},
		Message: "default/helloworld locked.",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test History
	hist, err := apiClient.History("", helloWorldSvc, time.Now().UTC(), -1, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) == 0 {
		t.Fatal("History should be longer than this: ", hist)
	}
	var hasLock bool
	for _, v := range hist {
		if strings.Contains(v.Data, "Locked") {
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
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("Request should have been ok but got %q, body:\n%v", resp.Status, body)
	}
}

func TestFluxsvc_Register(t *testing.T) {
	setup()
	defer teardown()

	_, err := httpdaemon.NewUpstream(&http.Client{}, "fluxd/test", "", router, ts.URL, mockPlatform, log.NewNopLogger()) // For ping and for
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
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("Request should have been ok but got %q, body:\n%v", resp.Status, body)
	}
}
