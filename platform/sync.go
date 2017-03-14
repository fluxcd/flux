package platform

import (
	"strings"
)

// Definitions for use in synchronising a platform with a git repo.

// This is really just to have a handle for reporting back; when,
// e.g., deleting a resource, it's the definition that matters. (The
// definition should certainly match the ID though, otherwise chaos or
// at least confusion will ensue)
type ResourceID string

func (id ResourceID) String() string {
	return string(id)
}

// Yep, resources are defined by opaque bytes. It's up to the platform
// at the other end to do the right thing.
type ResourceDef []byte

// The action(s) to take on a particular resource.
// This should just be done in order, i.e.,:
//  1. delete if something in Delete
//  2. create if something in Create
//  3. apply if something in Apply
type SyncAction struct {
	Delete ResourceDef
	Create ResourceDef
	Apply  ResourceDef
}

type SyncDef struct {
	// The actions to undertake
	Actions map[ResourceID]SyncAction
}

type SyncError map[ResourceID]error

func (err SyncError) Error() string {
	var errs []string
	for id, e := range err {
		errs = append(errs, id.String()+": "+e.Error())
	}
	return strings.Join(errs, "; ")
}
