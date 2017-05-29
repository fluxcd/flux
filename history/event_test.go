package history

import (
	"encoding/json"
	"fmt"
	"github.com/weaveworks/flux/update"
	"testing"
)

const (
	releaseID = "1"
)

func TestEvent_ParseReleaseMetaData(t *testing.T) {
	origEvent := Event{
		Type: EventRelease,
		Metadata: &ReleaseEventMetadata{
			Release: update.Release{
				ID: releaseID,
			},
		},
	}

	bytes, _ := json.Marshal(origEvent)

	e := Event{}
	err := e.UnmarshalJSON(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if e.Metadata.(ReleaseEventMetadata).Release.ID != releaseID {
		t.Fatal("Release.ID wasn't marshalled/unmarshalled")
	}
}

func TestEvent_ParseNormalMetadata(t *testing.T) {
	origEvent := Event{
		Type: EventSync,
		Metadata: &SyncEventMetadata{
			Revisions: []string{"1"},
		},
	}

	bytes, _ := json.Marshal(origEvent)

	e := Event{}
	err := e.UnmarshalJSON(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if e.Metadata == nil {
		t.Fatal("Hasn't been unmarshalled properly")
	}
	if fmt.Sprint(e.Metadata) != "map[revisions:[1]]" {
		t.Fatal("Expected metadata")
	}
}

func TestEvent_ParseNoMetadata(t *testing.T) {
	origEvent := Event{
		Type: EventLock,
	}

	bytes, _ := json.Marshal(origEvent)

	e := Event{}
	err := e.UnmarshalJSON(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if e.Metadata != nil {
		t.Fatal("Hasn't been unmarshalled properly")
	}
}
