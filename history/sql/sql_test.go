package sql

import (
	"flag"
	"io/ioutil"
	"net/url"
	"testing"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/db"
	"github.com/weaveworks/flux/history"
)

var (
	databaseSource = flag.String("database-source", "", `Database source name. The default is a temporary DB using ql`)
)

func mkDBFile(t *testing.T) string {
	f, err := ioutil.TempFile("", "fluxy-testdb")
	if err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func bailIfErr(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func newSQL(t *testing.T) history.DB {
	if *databaseSource == "" {
		*databaseSource = "file://" + mkDBFile(t)
	}

	u, err := url.Parse(*databaseSource)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = db.Migrate(*databaseSource, "../../db/migrations"); err != nil {
		t.Fatal(err)
	}

	db, err := NewSQL(db.DriverForScheme(u.Scheme), *databaseSource)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHistoryLog(t *testing.T) {
	instance := flux.InstanceID("instance")
	db := newSQL(t)
	defer db.Close()

	bailIfErr(t, db.LogEvent(instance, Event{
		ServiceIDs: []flux.ServiceID{flux.ServiceID("namespace/service")},
		Type:       "test",
		Message:    "event 1",
	}))
	bailIfErr(t, db.LogEvent(instance, Event{
		ServiceIDs: []flux.ServiceID{flux.ServiceID("namespace/other")},
		Type:       "test",
		Message:    "event 3",
	}))
	bailIfErr(t, db.LogEvent(instance, Event{
		ServiceIDs: []flux.ServiceID{flux.ServiceID("namespace/service")},
		Type:       "test",
		Message:    "event 2",
	}))

	es, err := db.EventsForService(instance, flux.ServiceID("namespace/service"), time.Now().UTC(), -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 {
		t.Fatalf("Expected 2 events, got %d\n", len(es))
	}
	checkInDescOrder(t, es)

	es, err = db.AllEvents(instance, time.Now().UTC(), -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 3 {
		t.Fatalf("Expected 3 events, got %#v\n", es)
	}
	checkInDescOrder(t, es)
}

func checkInDescOrder(t *testing.T, events []Event) {
	var last time.Time = time.Now()
	for _, event := range events {
		if event.StartedAt.After(last) {
			t.Fatalf("Events out of order: %+v > %s", event, last)
		}
		last = event.StartedAt
	}
}
