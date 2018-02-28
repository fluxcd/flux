package cluster

import (
	"errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/ssh"
)

var (
	ErrNoResourceFilesFoundForService             = errors.New("no resource file found for service")
	ErrMultipleResourceFilesFoundForService       = errors.New("multiple resource files found for service")
	ErrMultipleResourceDefinitionsFoundForService = errors.New("multiple resource definitions found for service")
)

// The things we can get from the running cluster. These used to form
// the Platform interface; but now we do more in the daemon so they
// are distinct interfaces.
type Cluster interface {
	// Get all of the services (optionally, from a specific namespace), excluding those
	AllControllers(maybeNamespace string) ([]Controller, error)
	SomeControllers([]flux.ResourceID) ([]Controller, error)
	Ping() error
	Export() ([]byte, error)
	Sync(SyncDef) error
	PublicSSHKey(regenerate bool) (ssh.PublicKey, error)
}

// Controller describes a platform resource that declares versioned images.
type Controller struct {
	ID     flux.ResourceID
	Status string // A status summary for display

	Containers ContainersOrExcuse
}

// A Container represents a container specification in a pod. The Name
// identifies it within the pod, and the Image says which image it's
// configured to run.
type Container struct {
	Name  string
	Image string
}

// Sometimes we care if we can't find the containers for a service,
// sometimes we just want the information we can get.
type ContainersOrExcuse struct {
	Excuse     string
	Containers []Container
}

func (s Controller) ContainersOrNil() []Container {
	return s.Containers.Containers
}

func (s Controller) ContainersOrError() ([]Container, error) {
	var err error
	if s.Containers.Excuse != "" {
		err = errors.New(s.Containers.Excuse)
	}
	return s.Containers.Containers, err
}

// These errors all represent logical problems with cluster
// configuration, and may be recoverable; e.g., it might be fine if a
// service does not have a matching RC/deployment.
var (
	ErrEmptySelector        = errors.New("empty selector")
	ErrWrongResourceKind    = errors.New("new definition does not match existing resource")
	ErrNoMatchingService    = errors.New("no matching service")
	ErrServiceHasNoSelector = errors.New("service has no selector")
	ErrNoMatching           = errors.New("no matching replication controllers or deployments")
	ErrMultipleMatching     = errors.New("multiple matching replication controllers or deployments")
	ErrNoMatchingImages     = errors.New("no matching images")
)
