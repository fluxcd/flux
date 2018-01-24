package cluster

import (
	"strings"
)

// Definitions for use in synchronising a cluster with a git repo.

// Yep, resources are defined by opaque bytes. It's up to the cluster
// at the other end to do the right thing.
type ResourceDef []byte

// The action(s) to take on a particular resource.
// This should just be done in order, i.e.,:
//  1. delete if something in Delete
//  2. apply if something in Apply
type SyncAction struct {
	// The ID is just a handle for labeling any error. No other
	// meaning is attached to it.
	ResourceID string
	Delete     ResourceDef
	Apply      ResourceDef
}

type SyncDef struct {
	// The actions to undertake
	Actions []SyncAction
}

type SyncError map[string]error

func (err SyncError) Error() string {
	var errs []string
	for id, e := range err {
		errs = append(errs, id+": "+e.Error())
	}
	return strings.Join(errs, "; ")
}
