// Shared main test code
package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/flux/pkg/http/client"
	"github.com/gorilla/mux"

	transport "github.com/fluxcd/flux/pkg/http"
	"github.com/fluxcd/flux/pkg/job"
)

func mockServiceOpts(trip *genericMockRoundTripper) *rootOpts {
	c := http.Client{
		Transport: trip,
	}
	mockAPI := client.New(&c, transport.NewAPIRouter(), "", "")
	return &rootOpts{
		API:     mockAPI,
		Timeout: 10 * time.Second,
	}
}

type genericMockRoundTripper struct {
	mockResponses  map[*mux.Route]interface{}
	requestHistory map[string]*http.Request
}

func (t *genericMockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var matched mux.RouteMatch
	var b []byte
	status := 404
	for k, v := range t.mockResponses {
		if k.Match(req, &matched) {
			queryParamsWithArrays := make(map[string]string, len(req.URL.Query()))
			for x, y := range req.URL.Query() {
				queryParamsWithArrays[x] = strings.Join(y, ",")
			}
			t.requestHistory[matched.Route.GetName()] = req
			b, _ = json.Marshal(v)
			status = 200
			break
		}
	}
	return &http.Response{
		StatusCode: status,
		Body:       ioutil.NopCloser(bytes.NewReader(b)),
	}, nil
}

func (t *genericMockRoundTripper) calledRequest(method string) *http.Request {
	return t.requestHistory[method]
}

func (t *genericMockRoundTripper) calledURL(method string) (u *url.URL) {
	return t.calledRequest(method).URL
}

func testArgs(t *testing.T, args []string, shouldErr bool, errMsg string) *genericMockRoundTripper {
	svc := newMockService()
	releaseClient := newWorkloadRelease(mockServiceOpts(svc))
	getKubeConfigContextNamespace = func(s string, c string) string { return s }

	// Run fluxctl release
	cmd := releaseClient.Command()
	cmd.SetOutput(ioutil.Discard)
	cmd.SetArgs(args)
	if err := cmd.Execute(); (err == nil) == shouldErr {
		if errMsg != "" {
			t.Fatalf("%s: %s", args, errMsg)
		} else {
			t.Fatalf("%s: %v", args, err)
		}
	}
	return svc
}

// The mocked service is actually a mocked http.RoundTripper
func newMockService() *genericMockRoundTripper {
	return &genericMockRoundTripper{
		mockResponses: map[*mux.Route]interface{}{
			transport.NewAPIRouter().Get("UpdateManifests"): job.ID("here-is-a-job-id"),
			transport.NewAPIRouter().Get("JobStatus"): job.Status{
				StatusString: job.StatusSucceeded,
			},
		},
		requestHistory: make(map[string]*http.Request),
	}
}
