package event

import (
	"encoding/json"
	"testing"

	"github.com/fluxcd/flux/pkg/update"
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

// TestEvent_ParseOldReleaseMetaData makes sure the parsing code can
// handle the older format events recorded against commits.
func TestEvent_ParseOldReleaseMetaData(t *testing.T) {
	// A minimal example of an old-style ReleaseEventMetadata. NB it
	// must have at least an entry for "spec", since otherwise the
	// JSON unmarshaller will not attempt to unparse the spec and
	// thereby invoke the specialised UnmarshalJSON.
	oldData := `
{
  "spec": {
    "serviceSpecs": ["<all>"]
  }
}
`
	var eventData ReleaseEventMetadata
	if err := json.Unmarshal([]byte(oldData), &eventData); err != nil {
		t.Fatal(err)
	}
	if eventData.Spec.Type != ReleaseImageSpecType {
		t.Error("did not set spec type to ReleaseImageSpecType")
	}
	if eventData.Spec.ReleaseImageSpec == nil {
		t.Error("did not set .ReleaseImageSpec as expected")
	}
	if eventData.Spec.ReleaseContainersSpec != nil {
		t.Error("unexpectedly set .ReleaseContainersSpec")
	}
	if len(eventData.Spec.ReleaseImageSpec.ServiceSpecs) != 1 {
		t.Error("expected service specs of len 1")
	}
}
