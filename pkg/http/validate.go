package http

import (
	"fmt"

	"github.com/gorilla/mux"
)

// ImplementsServer verifies that a given `*mux.Router` has handlers for
// all routes specified in `NewAPIRouter()`.
//
// We can't easily check whether a router implements the `api.Server`
// interface, as would be desired, so we rely on the knowledge that
// `*client.Client` implements `api.Server` while also depending on
// route name strings defined in this package.
//
// Returns an error if router doesn't fully implement `NewAPIRouter()`,
// nil otherwise.
func ImplementsServer(router *mux.Router) error {
	apiRouter := NewAPIRouter()
	return apiRouter.Walk(makeWalkFunc(router))
}

// makeWalkFunc creates a function which verifies that the route passed
// to it both exists in the router under test and has a handler attached.
func makeWalkFunc(router *mux.Router) mux.WalkFunc {
	return mux.WalkFunc(func(r *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		// Does a route with this name exist in router?
		route := router.Get(r.GetName())
		if route == nil {
			return fmt.Errorf("no route by name %q in router", r.GetName())
		}
		// Does the route have a handler?
		handler := route.GetHandler()
		if handler == nil {
			return fmt.Errorf("no handler for route %q in router", r.GetName())
		}
		return nil
	})
}
