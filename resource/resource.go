package resource

import (
	"github.com/weaveworks/flux"
)

// For the minute we just care about
type Resource interface {
	ResourceID() string             // name, to correlate with what's in the cluster
	ServiceIDs() []flux.ServiceID   // ServiceID returns the associated services for this resource
	Annotations() map[string]string // annotations that have been applied to the resource
	Source() string                 // where did this come from (informational)
	Bytes() []byte                  // the definition, for sending to platform.Sync
}
