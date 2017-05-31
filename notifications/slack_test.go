package notifications

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

	// It should send releases to slack
	if err := slackNotifyRelease(flux.NotifierConfig{
		HookURL:  server.URL,
		Username: "user1",
	}, exampleRelease(t), "test-error"); err != nil {
		t.Fatal(err)
	}
	if gotReq == nil {
		t.Fatal("Expected a request to slack to have been made")
	}

	// Req should be a post
	if gotReq.Method != "POST" {
		t.Errorf("Expected request method to be POST, but got %q", gotReq.Method)
	}

	body := map[string]string{}
	if err := json.NewDecoder(&bodyBuffer).Decode(&body); err != nil {
		t.Fatal(err)
	}
	for k, expectedV := range map[string]string{
		"username": "user1",
		"text":     "Release (test-user: this was to test notifications) all latest to default/helloworld. test-error. failed",
	} {
		if v, ok := body[k]; !ok || v != expectedV {
			t.Errorf("Expected %s to have been set to %q, but got: %q", k, expectedV, v)
		}
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

func TestSlackNotifierCustomTemplate(t *testing.T) {
	var gotReq *http.Request
	var bodyBuffer bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		io.Copy(&bodyBuffer, r.Body)
		w.WriteHeader(200)
	}))
	defer server.Close()

	// It should send releases to slack
	if err := slackNotifyRelease(flux.NotifierConfig{
		HookURL:         server.URL,
		ReleaseTemplate: "My custom template here",
	}, exampleRelease(t), "test-error"); err != nil {
		t.Fatal(err)
	}
	if gotReq == nil {
		t.Fatal("Expected a request to slack to have been made")
	}

	// Req should be a post
	if gotReq.Method != "POST" {
		t.Errorf("Expected request method to be POST, but got %q", gotReq.Method)
	}

	body := map[string]string{}
	if err := json.NewDecoder(&bodyBuffer).Decode(&body); err != nil {
		t.Fatal(err)
	}
	for k, expectedV := range map[string]string{
		"text": "My custom template here",
	} {
		if v, ok := body[k]; !ok || v != expectedV {
			t.Errorf("Expected %s to have been set to %q, but got: %q", k, expectedV, v)
		}
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
