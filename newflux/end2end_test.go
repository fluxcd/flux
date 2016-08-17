package flux

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEndToEnd(t *testing.T) {
	r := NewRouter()

	s := httptest.NewServer(NewHandler(NewServer(), r))
	defer s.Close()

	c := NewClient(http.DefaultClient, r, s.URL)

	_, err := c.ListServices()
	t.Logf("ListServices: %v", err)

	_, err = c.ListImages(ServiceSpec("namespace/service"))
	t.Logf("ListImages: %v", err)

	err = c.Release(ServiceSpec("namespace/service"), ImageSpec("image"))
	t.Logf("Release: %v", err)

	err = c.Automate(ServiceID("namespace/service"))
	t.Logf("Automate: %v", err)

	err = c.Deautomate(ServiceID("namespace/service"))
	t.Logf("Deautomate: %v", err)

	_, err = c.History(ServiceSpecAll)
	t.Logf("History: %v", err)
}
