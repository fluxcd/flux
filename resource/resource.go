package resource

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

// For the minute we just care about
type Resource interface {
	ResourceID() flux.ResourceID // name, to correlate with what's in the cluster
	Policy() policy.Set          // policy for this resource; e.g., whether it is locked, automated, ignored
	Source() string              // where did this come from (informational)
	Bytes() []byte               // the definition, for sending to cluster.Sync
}
