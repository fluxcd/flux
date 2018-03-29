package event

import (
	"encoding/json"
	"testing"

	"github.com/weaveworks/flux/update"
)

var (
	spec = update.ReleaseSpec{
		ImageSpec: update.ImageSpecLatest,
	}
	cause = update.Cause{
		User:    "test user",
		Message: "test message",
	}
)

func TestEvent_ParseReleaseMetaData(t *testing.T) {
	origEvent := Event{
		Type: EventRelease,
		Metadata: &ReleaseEventMetadata{
			Cause: cause,
			Spec:  spec,
		},
	}

	bytes, _ := json.Marshal(origEvent)

	e := Event{}
	err := e.UnmarshalJSON(bytes)
	if err != nil {
		t.Fatal(err)
	}
	switch r := e.Metadata.(type) {
	case *ReleaseEventMetadata:
		if r.Spec.ImageSpec != spec.ImageSpec ||
			r.Cause != cause {
			t.Fatal("Release event wasn't marshalled/unmarshalled")
		}
	default:
		t.Fatal("Wrong event type unmarshalled")
	}
}

func TestEvent_ParseNoMetadata(t *testing.T) {
	origEvent := Event{
		Type: EventUpdatePolicy,
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
