package http

import (
	"net/http"
	"testing"
)

func Test_NegotiateContentType(t *testing.T) {
	// For no accept header, you get your first choice
	want := "x-world/x-vrml"
	got := negotiateContentType(&http.Request{}, []string{want})
	if got != want {
		t.Errorf("First choice: Expected %q, got %q", want, got)
	}

	// If there's accept headers but none match, get ""
	h := http.Header{}
	h.Add("Accept", "application/json;q=1.0,text/html;q=0.9")
	h.Add("Accept", "text/plain")
	got = negotiateContentType(&http.Request{Header: h}, []string{want})
	if got != "" {
		t.Errorf("No matching: expected empty string, got %q", got)
	}

	// If there's accept headers that match, of equal quality (`q`),
	// return the first preference.
	h = http.Header{}
	h.Add("Accept", "application/json,x-world/x-vrml,text/html")
	got = negotiateContentType(&http.Request{Header: h}, []string{want, "application/json"})
	if got != want {
		t.Errorf("Equal quality: expected %q, got %q", want, got)
	}

	// If there's matching accept headers of different quality, pick
	// the highest quality match even if it's not first preference.
	h = http.Header{}
	h.Add("Accept", "application/json;q=0.5,text/html;q=1.0")
	got = negotiateContentType(&http.Request{Header: h}, []string{"application/json", "text/html"})
	if got != "text/html" {
		t.Errorf("Quality beats preference: expected %q, got %q", "text/html", got)
	}
}
