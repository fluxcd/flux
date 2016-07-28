package history

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
)

func mkDBFile() string {
	f, err := ioutil.TempFile("", "fluxy-testdb")
	if err != nil {
		panic(err)
	}
	return f.Name()
}

func TestHistoryCreate(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	_, err := NewSQL("ql", "file://"+mkDBFile(), logger)
	if err != nil {
		t.Fatal(err)
	}
}

func TestHistoryState(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	db, err := NewSQL("ql", "file://"+mkDBFile(), logger)
	if err != nil {
		t.Fatal(err)
	}

	s := ServiceState("new state")
	if err = db.ChangeState("namespace", "service", s); err != nil {
		t.Fatal(err)
	}

	h, err := db.EventsForService("namespace", "service")
	if err != nil {
		t.Fatal(err)
	}
	if h.State != s {
		t.Fatalf("Expected states to match, but %q != %q\n", h.State, s)
	}
}

func TestHistoryLog(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)
	db, err := NewSQL("ql", "file://"+mkDBFile(), logger)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.LogEvent("namespace", "service", "event 1"); err != nil {
		t.Fatal(err)
	}
	if err = db.LogEvent("namespace", "service", "event 2"); err != nil {
		t.Fatal(err)
	}
	h, err := db.EventsForService("namespace", "service")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Events) != 2 {
		t.Fatalf("Expected 2 events, got %d\n", len(h.Events))
	}

	hs, err := db.AllEvents("namespace")
	if err != nil {
		t.Fatal(err)
	}
	h, found := hs["service"]
	if !found {
		t.Fatalf("Did not find expected events for %q", "service")
	}
	if len(h.Events) != 2 {
		t.Fatalf("Expected 2 events, got %#v\n", h.Events)
	}
}
