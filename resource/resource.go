package resource

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

// For the minute we just care about
type Resource interface {
	ResourceID() string                              // name, to correlate with what's in the cluster
	ServiceIDs(map[string]Resource) []flux.ServiceID // ServiceIDs returns the associated services for this resource
	Policy() policy.Set                              // policy for this resource; e.g., whether it is locked, automated, ignored
	Source() string                                  // where did this come from (informational)
	Bytes() []byte                                   // the definition, for sending to platform.Sync
}
