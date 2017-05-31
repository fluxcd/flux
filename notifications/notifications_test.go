package notifications

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/update"
)

// Generate an example release
func exampleRelease(t *testing.T) update.Release {
	now := time.Now().UTC()
	img1a1, _ := flux.ParseImageID("img1:a1")
	img1a2, _ := flux.ParseImageID("img1:a2")
	return update.Release{
		CreatedAt: now.Add(-1 * time.Minute),
		StartedAt: now.Add(-30 * time.Second),
		EndedAt:   now.Add(-1 * time.Second),

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
		Result: update.Result{
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
	r.Spec.Kind = update.ReleaseKindPlan
	if err := Release(instance.Config{
		Settings: flux.UnsafeInstanceConfig{
			Slack: flux.NotifierConfig{
				HookURL: server.URL,
			},
		},
	}, r, ""); err != nil {
		t.Fatal(err)
	}
}
