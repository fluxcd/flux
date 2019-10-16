package daemon

import (
	"testing"

	"github.com/fluxcd/flux/pkg/http"
)

func TestRouterImplementsServer(t *testing.T) {
	router := NewRouter()
	// Calling NewHandler attaches handlers to the router
	NewHandler(nil, router)
	err := http.ImplementsServer(router)
	if err != nil {
		t.Error(err)
	}
}
