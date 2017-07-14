package notifications

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/service"
	"github.com/weaveworks/flux/service/instance"
	"github.com/weaveworks/flux/update"
)

// Generate an example release
func exampleRelease(t *testing.T) *history.ReleaseEventMetadata {
	img1a1, _ := flux.ParseImageID("img1:a1")
	img1a2, _ := flux.ParseImageID("img1:a2")
	exampleResult := update.Result{
		flux.ServiceID("default/helloworld"): {
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
	return &history.ReleaseEventMetadata{
		Cause: update.Cause{
			User:    "test-user",
			Message: "this was to test notifications",
		},
		Spec: update.ReleaseSpec{
			ServiceSpecs: []update.ServiceSpec{update.ServiceSpec("default/helloworld")},
			ImageSpec:    update.ImageSpecLatest,
			Kind:         update.ReleaseKindExecute,
			Excludes:     nil,
		},
		ReleaseEventCommon: history.ReleaseEventCommon{
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
	ev := history.Event{Metadata: r}
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
