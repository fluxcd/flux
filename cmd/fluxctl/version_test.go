package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Test that given a server that responds with 404 for a particular
// route, we get a version warning.
func TestUnknownVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	telltale := "endpoint not found"
	errout := &bytes.Buffer{}
	res := run([]string{"--url", server.URL, "list-services"}, errout)
	if res == 0 {
		t.Errorf("Expected non-zero return from main, got %d", res)
	}
	if !strings.Contains(errout.String(), telltale) {
		t.Fatalf("Expected %q in output to stderr, but it was not seen in %q", telltale, errout.String())
	}
}
