package notifications

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/service"
	"github.com/weaveworks/flux/service/instance"
	"github.com/weaveworks/flux/update"
)

// Generate an example release
func exampleRelease(t *testing.T) *event.ReleaseEventMetadata {
	img1a1, _ := flux.ParseImageID("img1:a1")
	img1a2, _ := flux.ParseImageID("img1:a2")
	exampleResult := update.Result{
		flux.MustParseResourceID("default/helloworld"): {
			Status: update.ReleaseStatusFailed,
			Error:  "overall-release-error",
			PerContainer: []update.ContainerUpdate{
				update.ContainerUpdate{
					Container: "container1",
					Current:   img1a1,
					Target:    img1a2,
				},
			},
		},
	}
	return &event.ReleaseEventMetadata{
		Cause: update.Cause{
			User:    "test-user",
			Message: "this was to test notifications",
		},
		Spec: update.ReleaseSpec{
			ServiceSpecs: []update.ResourceSpec{update.ResourceSpec("default/helloworld")},
			ImageSpec:    update.ImageSpecLatest,
			Kind:         update.ReleaseKindExecute,
			Excludes:     nil,
		},
		ReleaseEventCommon: event.ReleaseEventCommon{
			Result: exampleResult,
		},
	}
}

func TestRelease_DryRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Expected no http request to have been made")
	}))
	defer server.Close()

	// It should send releases to slack
	r := exampleRelease(t)
	ev := event.Event{Metadata: r}
	r.Spec.Kind = update.ReleaseKindPlan
	if err := Event(instance.Config{
		Settings: service.InstanceConfig{
			Slack: service.NotifierConfig{
				HookURL: server.URL,
			},
		},
	}, ev); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationEventsURL(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.WriteHeader(200)
	}))
	defer server.Close()

	var instanceID service.InstanceID = "2"
	var eventsURL = new(string)
	*eventsURL = server.URL + fmt.Sprintf("/%v/{eventType}", instanceID)
	eventType := "deploy"
	expectedPath := fmt.Sprintf("/%v/%v", instanceID, eventType)

	cfg := instance.Config{
		Settings: service.InstanceConfig{
			Slack: service.NotifierConfig{
				HookURL:         *eventsURL,
				ReleaseTemplate: "",
				NotifyEvents: []string{
					event.EventRelease,
					event.EventAutoRelease,
					event.EventSync,
				},
			},
		},
	}

	ev := event.Event{Metadata: exampleRelease(t), Type: event.EventRelease}

	err := Event(cfg, ev)

	if err != nil {
		t.Fatal(err)
	}

	if gotReq.URL.EscapedPath() != expectedPath {
		t.Fatalf("Expected: %v, Got: %v", expectedPath, gotReq.URL.String())
	}
}
