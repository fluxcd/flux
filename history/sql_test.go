package history

import (
	"io/ioutil"
	"testing"
)

func mkDBFile() string {
	f, err := ioutil.TempFile("", "fluxy-testdb")
	if err != nil {
		panic(err)
	}
	return f.Name()
}

func bailIfErr(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func TestHistoryLog(t *testing.T) {
	db, err := NewSQL("ql", "file://"+mkDBFile())
	if err != nil {
		t.Fatal(err)
	}

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
