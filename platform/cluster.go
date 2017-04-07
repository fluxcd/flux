package platform

import (
	"github.com/weaveworks/flux"
)

// The things we can get from the running cluster. These used to form
// the Platform interface; but now we do more in the daemon so they
// are distinct interfaces.
type Cluster interface {
	// Get all of the services (optionally, from a specific namespace), excluding those
	AllServices(maybeNamespace string) ([]Service, error)
	SomeServices([]flux.ServiceID) ([]Service, error)
	Ping() error
	Export() ([]byte, error)
	// For use by git sync
	Sync(SyncDef) error
	// Given a directory with manifest files, find which files define
	// which services.
	FindDefinedServices(path string) (map[flux.ServiceID][]string, error)
	UpdateDefinition(def []byte, newImageID flux.ImageID) ([]byte, error)
}
