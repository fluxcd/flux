package jobs

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/weaveworks/flux"
)

func TestJobEncodingDecoding(t *testing.T) {
	// Check it can serialize/deserialize release jobs
	now := time.Now().UTC()
	expected := Job{
		Instance: flux.InstanceID("instance"),
		ID:       NewJobID(),
		Queue:    DefaultQueue,
		Method:   ReleaseJob,
		Params: ReleaseJobParams{
			ServiceSpecs: []flux.ServiceSpec{flux.ServiceSpecAll},
			ImageSpec:    flux.ImageSpecLatest,
			Kind:         flux.ReleaseKindExecute,
		},
		ScheduledAt: now,
		Priority:    PriorityInteractive,
		Key:         "key1",
		Submitted:   now,
		Claimed:     now,
		Heartbeat:   now,
		Finished:    now,
		Result: flux.ReleaseResult{
			flux.ServiceID("hippo/birdy"): flux.ServiceResult{
				Status: flux.ReleaseStatusPending,
			},
		},
		Log:     []string{"log1"},
		Status:  "status",
		Done:    true,
		Success: false,
		Error:   &flux.BaseError{Err: errors.New("actual error"), Help: "helpful text"},
	}
	b, err := json.Marshal(expected)
	bailIfErr(t, err)
	var got Job
	bailIfErr(t, json.Unmarshal(b, &got))

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("got %q, expected %q", got, expected)
	}
}

func TestJobEncodingDecodingWithMissingFields(t *testing.T) {
	now := time.Now().UTC()
	input := Job{
		Instance:    flux.InstanceID("instance"),
		ID:          NewJobID(),
		Queue:       DefaultQueue,
		Method:      ReleaseJob,
		ScheduledAt: now,
		Priority:    PriorityInteractive,
		Key:         "key1",
		Submitted:   now,
		Claimed:     now,
		Heartbeat:   now,
		Finished:    now,
		Status:      "status",
		Done:        true,
		Success:     true,
	}
	b, err := json.Marshal(input)
	bailIfErr(t, err)
	var got Job
	bailIfErr(t, json.Unmarshal(b, &got))
}
