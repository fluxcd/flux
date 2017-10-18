package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"

	"io/ioutil"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/guid"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/http/client"
	httpdaemon "github.com/weaveworks/flux/http/daemon"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/service"
	"github.com/weaveworks/flux/service/bus"
	"github.com/weaveworks/flux/service/bus/nats"
	"github.com/weaveworks/flux/service/db"
	"github.com/weaveworks/flux/service/history"
	historysql "github.com/weaveworks/flux/service/history/sql"
	httpserver "github.com/weaveworks/flux/service/http"
	"github.com/weaveworks/flux/service/instance"
	instancedb "github.com/weaveworks/flux/service/instance/sql"
	"github.com/weaveworks/flux/service/server"
	"github.com/weaveworks/flux/update"
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
	apiClient *client.Client
)

const (
	helloWorldSvc = "default/helloworld"
	ver           = "123"
	id            = service.InstanceID("californian-hotel-76")
)

func setup(t *testing.T) {
	databaseSource := "file://fluxy.db"
	databaseMigrationsDir, _ := filepath.Abs("../../service/db/migrations")
	var dbDriver string
	{
		db.Migrate(databaseSource, databaseMigrationsDir)
		dbDriver = db.DriverForScheme("file")
	}

	// Message bus
	messageBus, err := nats.NewMessageBus("nats://localhost:4222", bus.MetricsImpl)
	if err != nil {
		t.Fatal(err)
	}

	imageID, _ := flux.ParseImageID("quay.io/weaveworks/helloworld:v1")
	mockPlatform = &remote.MockPlatform{
		ListServicesAnswer: []flux.ControllerStatus{
			flux.ControllerStatus{
				ID:     flux.MustParseResourceID(helloWorldSvc),
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
			flux.ControllerStatus{},
		},
		ListImagesAnswer: []flux.ImageStatus{
			flux.ImageStatus{
				ID: flux.MustParseResourceID(helloWorldSvc),
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
				ID: flux.MustParseResourceID("a/another"),
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
	ctx := context.Background()
	messageBus.Subscribe(ctx, id, mockPlatform, done)
	if err := messageBus.AwaitPresence(id, 5*time.Second); err != nil {
		t.Errorf("Timed out waiting for presence of mockPlatform")
	}

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
	router = httpserver.NewServiceRouter()
	handler := httpserver.NewHandler(apiServer, router, log.NewNopLogger())
	handler = addInstanceIDHandler(handler)
	ts = httptest.NewServer(handler)
	apiClient = client.New(http.DefaultClient, router, ts.URL, "")
}

func addInstanceIDHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Add(httpserver.InstanceIDHeaderKey, string(id))
		handler.ServeHTTP(w, r)
	})
}

func teardown() {
	ts.Close()
}

func TestFluxsvc_ListServices(t *testing.T) {
	setup(t)
	defer teardown()

	ctx := context.Background()

	// Test ListServices
	svcs, err := apiClient.ListServices(ctx, "default")
	if err != nil {
		t.Error(err)
	}
	if len(svcs) != 2 {
		t.Error("Expected there to be two services")
	}
	if svcs[0].ID.String() != helloWorldSvc && svcs[1].ID.String() != helloWorldSvc {
		t.Errorf("Expected one of the services to be %q", helloWorldSvc)
	}

	// Test that `namespace` argument is mandatory
	u, err := transport.MakeURL(ts.URL, router, "ListServices")
	if err != nil {
		t.Error(err)
	}
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request should result in 404, but got: %q", resp.Status)
	}
}

// Note that this test will reach out to docker hub to check the images
// associated with alpine
func TestFluxsvc_ListImages(t *testing.T) {
	setup(t)
	defer teardown()

	ctx := context.Background()

	// Test ListImages
	imgs, err := apiClient.ListImages(ctx, update.ResourceSpecAll)
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
	imgs, err = apiClient.ListImages(ctx, helloWorldSvc)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Error("Expected two sets of images")
	}
	if len(imgs[0].Containers) == 0 {
		t.Error("Expected >1 containers")
	}

	// Test that `service` argument is mandatory
	u, err := transport.MakeURL(ts.URL, router, "ListImages")
	if err != nil {
		t.Error(err)
	}
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request should result in 404, but got: %s", resp.Status)
	}
}

func TestFluxsvc_Release(t *testing.T) {
	setup(t)
	defer teardown()

	ctx := context.Background()

	mockPlatform.UpdateManifestsAnswer = job.ID(guid.New())
	mockPlatform.JobStatusAnswer = job.Status{
		StatusString: job.StatusQueued,
	}

	// Test UpdateImages
	r, err := apiClient.UpdateImages(ctx, update.ReleaseSpec{
		ImageSpec:    "alpine:latest",
		Kind:         "execute",
		ServiceSpecs: []update.ResourceSpec{helloWorldSvc},
	}, update.Cause{})
	if err != nil {
		t.Fatal(err)
	}
	if r != mockPlatform.UpdateManifestsAnswer {
		t.Error("%q != %q", r, mockPlatform.UpdateManifestsAnswer)
	}

	// Test GetRelease
	res, err := apiClient.JobStatus(ctx, r)
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
		t.Fatalf("Path should 404, got: %s", resp.Status)
	}
}

func TestFluxsvc_History(t *testing.T) {
	setup(t)
	defer teardown()

	ctx := context.Background()

	err := apiClient.LogEvent(ctx, event.Event{
		Type: event.EventLock,
		ServiceIDs: []flux.ResourceID{
			flux.MustParseResourceID(helloWorldSvc),
		},
		Message: "default/helloworld locked.",
	})
	if err != nil {
		t.Fatal(err)
	}

	var hist []history.Entry
	err = apiClient.Get(ctx, &hist, "History", "service", helloWorldSvc)
	if err != nil {
		t.Error(err)
	} else {
		var hasLock bool
		for _, v := range hist {
			if strings.Contains(v.Data, "Locked") {
				hasLock = true
				break
			}
		}
		if !hasLock {
			t.Errorf("History hasn't recorded a lock ", hist)
		}
	}

	// Test `service` argument is mandatory
	u, _ := transport.MakeURL(ts.URL, router, "History")
	resp, err := http.Get(u.String())
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Request should result in 404, got: %s", resp.Status)
	}
}

func TestFluxsvc_Status(t *testing.T) {
	setup(t)
	defer teardown()

	ctx := context.Background()

	// Test Status
	var status service.Status
	err := apiClient.Get(ctx, &status, "Status")
	if err != nil {
		t.Fatal(err)
	}
	if status.Fluxsvc.Version != ver {
		t.Fatalf("Expected %q, got %q", ver, status.Fluxsvc.Version)
	}
}

func TestFluxsvc_Ping(t *testing.T) {
	setup(t)
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
		t.Fatal("Request should have been ok but got %q, body:\n%q", resp.Status, string(body))
	}
}

func TestFluxsvc_Register(t *testing.T) {
	setup(t)
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
		t.Fatalf("Request should have been ok but got %q, body:\n%q", resp.Status, string(body))
	}
}
