package sql

import (
	"flag"
	"io/ioutil"
	"testing"
	"time"

	"github.com/weaveworks/fluxy/history"
)

var (
	databaseDriver = flag.String("database-driver", "ql", `Database driver name, e.g., "postgres"; the default is an in-memory DB`)
	databaseSource = flag.String("database-source", "", `Database source name; specific to the database driver (--database-driver) used. The default is an arbitrary, in-memory DB name`)
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
	if *databaseDriver == "ql" && *databaseSource == "" {
		*databaseSource = "file://" + mkDBFile(t)
	}
	db, err := NewSQL(*databaseDriver, *databaseSource)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHistoryLog(t *testing.T) {
	db := newSQL(t)
	defer db.Close()

	bailIfErr(t, db.LogEvent("namespace", "service", "event 1"))
	bailIfErr(t, db.LogEvent("namespace", "other", "event 3"))
	bailIfErr(t, db.LogEvent("namespace", "service", "event 2"))

	es, err := db.EventsForService("namespace", "service")
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 {
		t.Fatalf("Expected 2 events, got %d\n", len(es))
	}
	checkInDescOrder(t, es)

	es, err = db.AllEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 3 {
		t.Fatalf("Expected 3 events, got %#v\n", es)
	}
	checkInDescOrder(t, es)
}

func checkInDescOrder(t *testing.T, events []history.Event) {
	var last time.Time = time.Now()
	for _, event := range events {
		if event.Stamp.After(last) {
			t.Fatalf("Events out of order: %+v > %s", event, last)
		}
		last = event.Stamp
	}
}
