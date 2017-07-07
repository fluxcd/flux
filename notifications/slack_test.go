package notifications

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/update"
)

func TestSlackNotifier(t *testing.T) {
	var gotReq *http.Request
	var bodyBuffer bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		io.Copy(&bodyBuffer, r.Body)
		w.WriteHeader(200)
	}))
	defer server.Close()

	release := exampleRelease(t)

	// It should send releases to slack
	if err := slackNotifyRelease(flux.NotifierConfig{
		HookURL:  server.URL,
		Username: "user1",
	}, release, "test-error"); err != nil {
		t.Fatal(err)
	}
	if gotReq == nil {
		t.Fatal("Expected a request to slack to have been made")
	}

	// Req should be a post
	if gotReq.Method != "POST" {
		t.Errorf("Expected request method to be POST, but got %q", gotReq.Method)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(&bodyBuffer).Decode(&body); err != nil {
		t.Fatal(err)
	}

	// compile a result output to compare to
	var resultOut bytes.Buffer
	update.PrintResults(&resultOut, release.Result, false)
	expected := map[string]interface{}{
		"username": "user1",
		"text":     "Release all latest to default/helloworld.",
		"attachments": []interface{}{
			map[string]interface{}{
				"text":     "test-error",
				"fallback": "test-error",
				"color":    "warning",
			},
			map[string]interface{}{
				"text":        "this was to test notifications",
				"author_name": "test-user",
			},
			map[string]interface{}{
				"text":      "```" + resultOut.String() + "```",
				"color":     "warning",
				"mrkdwn_in": []interface{}{"text"},
			},
		},
	}
	if !reflect.DeepEqual(body, expected) {
		t.Errorf("Expected:\n%#v\nGot:\n%#v\n", expected, body)
	}
}

func TestSlackNotifierDryRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Expected no request to slack to have been made")
	}))
	defer server.Close()

	// It should send releases to slack
	release := exampleRelease(t)
	release.Spec.Kind = update.ReleaseKindPlan
	if err := slackNotifyRelease(flux.NotifierConfig{HookURL: server.URL}, release, "test-error"); err != nil {
		t.Fatal(err)
	}
}

func TestSlackNotifierErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// It should get an error back from slack
	err := slackNotifyRelease(flux.NotifierConfig{HookURL: server.URL}, exampleRelease(t), "test-error")
	if err == nil {
		t.Fatal("Expected an error back")
	}
	expected := "500 Internal Server Error from Slack (Internal Server Error)"
	if err.Error() != expected {
		t.Fatalf("Expected error back: %q, got %q", expected, err.Error())
	}
}
