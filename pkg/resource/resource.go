package resource

import (
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/policy"
)

// For the minute we just care about
type Resource interface {
	ResourceID() ID       // name, to correlate with what's in the cluster
	Policies() policy.Set // policy for this resource; e.g., whether it is locked, automated, ignored
	Source() string       // where did this come from (informational)
	Bytes() []byte        // the definition, for sending to cluster.Sync
}

type Container struct {
	Name  string
	Image image.Ref
}

type Workload interface {
	Resource
	Containers() []Container
	// SetContainerImage mutates this workload so that the container
	// named has the image given. This is not expected to have an
	// effect on any underlying file or cluster resource.
	SetContainerImage(container string, ref image.Ref) error
}
