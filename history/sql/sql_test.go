package sql

import (
	"flag"
	"io/ioutil"
	"testing"

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
	bailIfErr(t, db.LogEvent("namespace", "service", "event 2"))
	bailIfErr(t, db.LogEvent("namespace", "other", "event 3"))

	es, err := db.EventsForService("namespace", "service")
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 {
		t.Fatalf("Expected 2 events, got %d\n", len(es))
	}

	es, err = db.AllEvents("namespace")
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 3 {
		t.Fatalf("Expected 3 events, got %#v\n", es)
	}
}
