package http

import (
	"github.com/gorilla/mux"
)

// NewRouter creates a new router instance, registers all API routes
// and returns it.
func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.NewRoute().Name(SyncGit).Methods("POST").Path("/v1/sync-git")
	return r
}
