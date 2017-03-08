package main //+integration
import (
	"github.com/gorilla/mux"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/jobs"
	"testing"
)

func TestCheckReleaseCommand_CLIConversion(t *testing.T) {
	for _, v := range []struct {
		args           []string
		expectedParams map[string]string
	}{
		{[]string{"--release-id=1", "--no-follow=false"}, map[string]string{
			"id": "1",
		}},
		{[]string{"--release-id=1", "--no-follow=true"}, map[string]string{
			"id": "1",
		}},
	} {
		svc := testCheckReleaseArgs(t, v.args, false, "")

		// Check that PostRelease was called with correct args
		method := "GetRelease"
		if calledURL(method, svc.requestHistory) == nil {
			t.Fatalf("Expecting fluxctl to request %q, but did not.", method)
		}
		vars := calledRequest(method, svc.requestHistory).Vars
		for kk, vv := range v.expectedParams {
			assertString(t, vv, vars[kk])
		}
	}
}

func TestCheckReleaseCommand_InputFailures(t *testing.T) {
	for _, v := range []struct {
		args []string
		msg  string
	}{
		{[]string{"this is an argument"}, "Should error because no args"},
		{[]string{"--no-follow=true"}, "Should fail because we didn't pass an id"},
	} {
		testCheckReleaseArgs(t, v.args, true, v.msg)
	}

}

func testCheckReleaseArgs(t *testing.T, args []string, shouldErr bool, errMsg string) *genericMockRoundTripper {
	svc := &genericMockRoundTripper{
		mockResponses: map[*mux.Route]interface{}{
			transport.NewRouter().Get("GetRelease").Queries("id", "1"): jobs.Job{
				Done: true,
				ID:   "1",
				Params: jobs.ReleaseJobParams{
					Kind: "test",
				},
				Method: jobs.ReleaseJob,
			},
		},
	}
	client := newServiceCheckRelease(mockServiceOpts(svc))

	cmd := client.Command()
	cmd.SetArgs(args)
	if err := cmd.Execute(); (err == nil) == shouldErr {
		if errMsg != "" {
			t.Fatal(errMsg)
		} else {
			t.Fatal(err)
		}
	}
	return svc
}
