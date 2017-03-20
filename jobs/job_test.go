package jobs

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/weaveworks/flux"
	"strings"
)

var (
	noResultExample = `{
  "instanceID": "<default-instance-id>",
  "id": "46ce39e6-711c-e2d2-6a60-51306f111040",
  "queue": "release",
  "method": "release",
  "params": {
    "ServiceSpec": "default/helloworld",
    "ServiceSpecs": null,
    "ImageSpec": "<all latest>",
    "Kind": "execute",
    "Excludes": null
  },
  "scheduled_at": "2017-02-24T14:56:03.224413935Z",
  "priority": 200,
  "submitted": "2017-02-24T14:56:03.224413935Z",
  "claimed": "2017-02-24T14:56:03.393622435Z",
  "heartbeat": "0001-01-01T00:00:00Z",
  "finished": "2017-02-24T14:56:04.111071358Z",
  "log": [
    "Queued.",
    "Calculating release actions.",
    "Release latest images to default/helloworld",
    "Service default/helloworld image quay.io/weaveworks/helloworld:master-9a16ff945b9e is already the latest one; skipping.",
    "Service default/helloworld image quay.io/weaveworks/sidecar:master-a000002 is already the latest one; skipping.",
    "All selected services are running the requested images. Nothing to do."
  ],
  "status": "Complete.",
  "done": true,
  "success": true
}`
	noParamsExample = `{
  "instanceID": "<default-instance-id>",
  "id": "46ce39e6-711c-e2d2-6a60-51306f111040",
  "queue": "release",
  "method": "release",
  "scheduled_at": "2017-02-24T14:56:03.224413935Z",
  "priority": 200,
  "submitted": "2017-02-24T14:56:03.224413935Z",
  "claimed": "2017-02-24T14:56:03.393622435Z",
  "heartbeat": "0001-01-01T00:00:00Z",
  "finished": "2017-02-24T14:56:04.111071358Z",
  "log": [
    "Queued.",
    "Calculating release actions.",
    "Release latest images to default/helloworld",
    "Service default/helloworld image quay.io/weaveworks/helloworld:master-9a16ff945b9e is already the latest one; skipping.",
    "Service default/helloworld image quay.io/weaveworks/sidecar:master-a000002 is already the latest one; skipping.",
    "All selected services are running the requested images. Nothing to do."
  ],
  "status": "Complete.",
  "done": true,
  "success": true,
  "result":{"id":{"Status":"ok","Error":"","PerContainer":null}}
}`
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
			ReleaseSpec: flux.ReleaseSpec{
				ServiceSpecs: []flux.ServiceSpec{flux.ServiceSpecAll},
				ImageSpec:    flux.ImageSpecLatest,
				Kind:         flux.ReleaseKindExecute,
			},
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
		t.Errorf("got %+v\nexpected %+v", got, expected)
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

func TestJob_NoParams(t *testing.T) {
	r := strings.NewReader(noParamsExample)
	var res Job
	if err := json.NewDecoder(r).Decode(&res); err != nil {
		t.Fatal(err)
	}
}

func TestJob_NoResult(t *testing.T) {
	r := strings.NewReader(noResultExample)
	var res Job
	if err := json.NewDecoder(r).Decode(&res); err != nil {
		t.Fatal(err)
	}
}
