package main //+integration

import (
	"testing"

	"github.com/gorilla/mux"

	"github.com/weaveworks/flux"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/jobs"
)

func TestReleaseCommand_CLIConversion(t *testing.T) {
	for _, v := range []struct {
		args           []string
		expectedParams map[string]string
	}{
		{[]string{"--update-all-images", "--all"}, map[string]string{
			"service": string(flux.ServiceSpecAll),
			"image":   string(flux.ImageSpecLatest),
			"kind":    string(flux.ReleaseKindExecute),
		}},
		{[]string{"--update-all-images", "--all", "--dry-run"}, map[string]string{
			"service": string(flux.ServiceSpecAll),
			"image":   string(flux.ImageSpecLatest),
			"kind":    string(flux.ReleaseKindPlan),
		}},
		{[]string{"--no-update", "--all"}, map[string]string{
			"service": string(flux.ServiceSpecAll),
			"image":   string(flux.ImageSpecNone),
			"kind":    string(flux.ReleaseKindExecute),
		}},
		{[]string{"--update-image=alpine:latest", "--all"}, map[string]string{
			"service": string(flux.ServiceSpecAll),
			"image":   "alpine:latest",
			"kind":    string(flux.ReleaseKindExecute),
		}},
		{[]string{"--update-all-images", "--service=default/flux"}, map[string]string{
			"service": "default/flux",
			"image":   string(flux.ImageSpecLatest),
			"kind":    string(flux.ReleaseKindExecute),
		}},
		{[]string{"--update-all-images", "--all", "--exclude=default/test,default/yeah"}, map[string]string{
			"service": string(flux.ServiceSpecAll),
			"image":   string(flux.ImageSpecLatest),
			"kind":    string(flux.ReleaseKindExecute),
			"exclude": "default/test,default/yeah",
		}},
	} {
		svc := testArgs(t, v.args, false, "")

		// Check that PostRelease was called with correct args
		method := "PostRelease"
		if calledURL(method, svc.requestHistory) == nil {
			t.Fatalf("Expecting fluxctl to request %q, but did not.", method)
		}
		vars := calledRequest(method, svc.requestHistory).Vars
		for kk, vv := range v.expectedParams {
			assertString(t, vv, vars[kk])
		}

		// Check that GetRelease was polled for status
		method = "GetRelease"
		if calledURL("GetRelease", svc.requestHistory) == nil {
			t.Fatalf("Expecting fluxctl to request %q, but did not.", method)
		}
	}
}

func TestReleaseCommand_NoFollow(t *testing.T) {
	svc := testArgs(t, []string{"--update-all-images", "--all", "--no-follow"}, false, "")
	// Check that GetRelease was not polled
	method := "GetRelease"
	if calledURL(method, svc.requestHistory) != nil {
		t.Fatalf("In no-follow mode so shouldn't have called %q", method)
	}

}

func TestReleaseCommand_InputFailures(t *testing.T) {
	for _, v := range []struct {
		args []string
		msg  string
	}{
		{[]string{}, "Should error when no args"},
		{[]string{"--all"}, "Should error when not specifying image spec"},
		{[]string{"--all", "--update-image=alpine"}, "Should error with invalid image spec"},
		{[]string{"--update-all-images"}, "Should error when not specifying service spec"},
		{[]string{"--service=invalid&service", "--update-all-images"}, "Should error with invalid service"},
		{[]string{"subcommand"}, "Should error when given subcommand"},
	} {
		testArgs(t, v.args, true, v.msg)
	}

}

// The mocked service is actually a mocked http.RoundTripper
func newMockService() *genericMockRoundTripper {
	return &genericMockRoundTripper{
		mockResponses: map[*mux.Route]interface{}{
			transport.NewRouter().Get("PostRelease"): transport.PostReleaseResponse{
				Status:    "ok",
				ReleaseID: "1",
			},
			transport.NewRouter().Get("GetRelease"): jobs.Job{
				Done: true,
				ID:   "1",
				Params: jobs.ReleaseJobParams{
					ReleaseSpec: flux.ReleaseSpec{
						Kind: "test",
					},
				},
				Method: jobs.ReleaseJob,
			},
		},
	}
}
