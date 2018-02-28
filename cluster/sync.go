package cluster

import (
	"strings"

	"github.com/weaveworks/flux/resource"
)

// Definitions for use in synchronising a cluster with a git repo.

// SyncAction represents either the deletion or application (create or
// update) of a resource.
type SyncAction struct {
	Delete resource.Resource // ) one of these
	Apply  resource.Resource // )
}

type SyncDef struct {
	// The actions to undertake
	Actions []SyncAction
}

type ResourceError struct {
	resource.Resource
	Error error
}

type SyncError []ResourceError

func (err SyncError) Error() string {
	var errs []string
	for _, e := range err {
		errs = append(errs, e.ResourceID().String()+": "+e.Error.Error())
	}
	return strings.Join(errs, "; ")
}
