// Shared main test code
package main

import (
	"net/http"

	transport "github.com/weaveworks/flux/http"

	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/weaveworks/flux/http/client"
)

func mockServiceOpts(trip *genericMockRoundTripper) *serviceOpts {
	c := http.Client{
		Transport: trip,
	}
	mockAPI := client.New(&c, transport.NewRouter(), "", "")
	return &serviceOpts{
		rootOpts: &rootOpts{
			API: mockAPI,
		},
	}
}

type genericMockRoundTripper struct {
	mockResponses  map[*mux.Route]interface{}
	requestHistory []mux.RouteMatch
}

func (t *genericMockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var matched mux.RouteMatch
	var b []byte
	status := 404
	fmt.Println(req.URL.String())
	for k, v := range t.mockResponses {
		if k.Match(req, &matched) {
			queryParamsWithArrays := make(map[string]string, len(req.URL.Query()))
			for x, y := range req.URL.Query() {
				queryParamsWithArrays[x] = strings.Join(y, ",")
			}
			matched.Vars = queryParamsWithArrays
			t.requestHistory = append(t.requestHistory, matched)
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

func assertString(t *testing.T, s1, s2 string) {
	if s1 != s2 {
		t.Fatalf("Expected %q but got %q", s1, s2)
	}
}

func calledRequest(method string, calls []mux.RouteMatch) (matched mux.RouteMatch) {
	for _, r := range calls {
		if method == r.Route.GetName() {
			matched = r
			break
		}
	}
	return
}

func calledURL(method string, calls []mux.RouteMatch) (u *url.URL) {
	r := calledRequest(method, calls)
	var vars []string
	for ik, iv := range r.Vars {
		vars = append(vars, ik)
		vars = append(vars, iv)
	}
	if r.Route != nil {
		u, _ = r.Route.URL(vars...)
	}
	return u
}

func testArgs(t *testing.T, args []string, shouldErr bool, errMsg string) *genericMockRoundTripper {
	svc := newMockService()
	releaseClient := newServiceRelease(mockServiceOpts(svc))

	// Run fluxctl release
	cmd := releaseClient.Command()
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
