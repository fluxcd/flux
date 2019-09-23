package cluster

import (
	"strings"

	"github.com/fluxcd/flux/pkg/resource"
)

// Definitions for use in synchronising a cluster with a git repo.

// SyncSet groups the set of resources to be updated. Usually this is
// the set of resources found in a git repo; in any case, it must
// represent the complete set of resources, as garbage collection will
// assume missing resources should be deleted. The name is used to
// distinguish the resources from a set from other resources -- e.g.,
// cluster resources not marked as belonging to a set will not be
// deleted by garbage collection.
type SyncSet struct {
	Name      string
	Resources []resource.Resource
}

type ResourceError struct {
	ResourceID resource.ID
	Source     string
	Error      error
}

type SyncError []ResourceError

func (err SyncError) Error() string {
	var errs []string
	for _, e := range err {
		errs = append(errs, e.ResourceID.String()+": "+e.Error.Error())
	}
	return strings.Join(errs, "; ")
}
