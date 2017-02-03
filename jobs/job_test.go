package jobs

import (
	"encoding/json"
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
		Log:         []string{"log1"},
		Status:      "status",
		Done:        true,
		Success:     true,
	}
	b, err := json.Marshal(expected)
	bailIfErr(t, err)
	var got Job
	bailIfErr(t, json.Unmarshal(b, &got))

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("got %q, expected %q", got, expected)
	}
}
