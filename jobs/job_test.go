package jobs

import (
	"encoding/json"
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
			ServiceSpec: flux.ServiceSpecAll,
			ImageSpec:   flux.ImageSpecLatest,
			Kind:        flux.ReleaseKindExecute,
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

	if got.Instance != expected.Instance {
		t.Errorf("got.Instance != expected.Instance: got %q, expected %q", got.Instance, expected.Instance)
	}
	if got.ID != expected.ID {
		t.Errorf("got.ID != expected.ID: got %q, expected %q", got.ID, expected.ID)
	}
	if got.Queue != expected.Queue {
		t.Errorf("got.Queue != expected.Queue: got %q, expected %q", got.Queue, expected.Queue)
	}
	if got.Method != expected.Method {
		t.Errorf("got.Method != expected.Method: got %q, expected %q", got.Method, expected.Method)
	}
	switch got.Params.(type) {
	case ReleaseJobParams:
		// noop
	default:
		t.Errorf("decoded params were not a ReleaseJobs params. got: %T", got.Params)
	}
	if got.ScheduledAt != expected.ScheduledAt {
		t.Errorf("got.ScheduledAt != expected.ScheduledAt: got %q, expected %q", got.ScheduledAt, expected.ScheduledAt)
	}
	if got.Priority != expected.Priority {
		t.Errorf("got.Priority != expected.Priority: got %q, expected %q", got.Priority, expected.Priority)
	}
	if got.Key != expected.Key {
		t.Errorf("got.Key != expected.Key: got %q, expected %q", got.Key, expected.Key)
	}
	if got.Submitted != expected.Submitted {
		t.Errorf("got.Submitted != expected.Submitted: got %q, expected %q", got.Submitted, expected.Submitted)
	}
	if got.Claimed != expected.Claimed {
		t.Errorf("got.Claimed != expected.Claimed: got %q, expected %q", got.Claimed, expected.Claimed)
	}
	if got.Heartbeat != expected.Heartbeat {
		t.Errorf("got.Heartbeat != expected.Heartbeat: got %q, expected %q", got.Heartbeat, expected.Heartbeat)
	}
	if got.Finished != expected.Finished {
		t.Errorf("got.Finished != expected.Finished: got %q, expected %q", got.Finished, expected.Finished)
	}
	if len(got.Log) != len(expected.Log) || got.Log[0] != expected.Log[0] {
		t.Errorf("got.Log != expected.Log: got %q, expected %q", got.Log, expected.Log)
	}
	if got.Status != expected.Status {
		t.Errorf("got.Status != expected.Status: got %q, expected %q", got.Status, expected.Status)
	}
	if got.Done != expected.Done {
		t.Errorf("got.Done != expected.Done: got %q, expected %q", got.Done, expected.Done)
	}
	if got.Success != expected.Success {
		t.Errorf("got.Success != expected.Success: got %q, expected %q", got.Success, expected.Success)
	}
}
