package event

import (
	"encoding/json"
	"testing"

	"github.com/weaveworks/flux/update"
)

var (
	spec = update.ReleaseImageSpec{
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
			Spec: ReleaseSpec{
				Type:             ReleaseImageSpecType,
				ReleaseImageSpec: &spec,
			},
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
		if r.Spec.ReleaseImageSpec.ImageSpec != spec.ImageSpec ||
			r.Cause != cause {
			t.Fatal("Release event wasn't marshalled/unmarshalled")
		}
	default:
		t.Fatal("Wrong event type unmarshalled")
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
