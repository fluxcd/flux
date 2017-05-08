package notifications

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
)

// Generate an example release
func exampleRelease(t *testing.T) flux.Release {
	now := time.Now().UTC()
	img1a1, _ := flux.ParseImageID("img1:a1")
	img1a2, _ := flux.ParseImageID("img1:a2")
	return flux.Release{
		ID:        flux.NewReleaseID(),
		CreatedAt: now.Add(-1 * time.Minute),
		StartedAt: now.Add(-30 * time.Second),
		EndedAt:   now.Add(-1 * time.Second),
		Done:      true,
		Priority:  100,
		Status:    flux.ReleaseStatusFailed,
		Log:       []string{string(flux.ReleaseStatusFailed)},

		Spec: flux.ReleaseSpec{
			ServiceSpecs: []flux.ServiceSpec{flux.ServiceSpec("default/helloworld")},
			ImageSpec:    flux.ImageSpecLatest,
			Kind:         flux.ReleaseKindExecute,
			Excludes:     nil,
			Cause: flux.ReleaseCause{
				User:    "test-user",
				Message: "this was to test notifications",
			},
		},
		Result: flux.ReleaseResult{
			flux.ServiceID("default/helloworld"): {
				Status: flux.ReleaseStatusFailed,
				Error:  "overall-release-error",
				PerContainer: []flux.ContainerUpdate{
					flux.ContainerUpdate{
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
	r.Spec.Kind = flux.ReleaseKindPlan
	if err := Release(instance.Config{
		Settings: flux.UnsafeInstanceConfig{
			Slack: flux.NotifierConfig{
				HookURL: server.URL,
			},
		},
	}, r, nil); err != nil {
		t.Fatal(err)
	}
}
