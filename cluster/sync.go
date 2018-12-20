package cluster

import (
	"strings"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/resource"
)

// Definitions for use in synchronising a cluster with a git repo.

// SyncStack groups a set of resources to be updated. The purpose of
// the grouping is to limit the "blast radius" of changes. For
// example, if we calculate a checksum for each stack and annotate the
// resources within it, any single change will affect only the
// resources in the same stack, meaning fewer things to annotate. (So
// why not do these individually? This, too, can be expensive, since
// it involves examining each resource individually).
type SyncStack struct {
	Name      string
	Resources []resource.Resource
}

type SyncDef struct {
	// The applications to undertake
	Stacks []SyncStack
}

type ResourceError struct {
	ResourceID flux.ResourceID
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
