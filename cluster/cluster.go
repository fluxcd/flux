package cluster

import (
	"errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/ssh"
)

// The things we can get from the running cluster. These used to form
// the remote.Platform interface; but now we do more in the daemon so they
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

// ControllerPodStatus describes detailed controller pod status
type ControllerPodStatus struct {
	// Status of controller pod
	Status string
	// Desired number of pods as defined in spec.
	Desired int32
	// Updated number of pods that are on the desired pod spec.
	Updated int32
	// Ready number of pods that are on the desired pod spec and ready.
	Ready int32
	// Outdated number of pods that are on a different pod spec.
	Outdated int32
	// PodConditions represents the latest available observations of a controller's current state.
	PodConditions []PodCondition
}

// PodCondition describes pod condition
type PodCondition struct {
	Name string
	// A human readable message indicating details about why the pod is in this condition.
	Message string
	// A brief CamelCase message indicating details about why the pod is in this state.
	Reason     string
	Conditions []Condition
}

// Condition describes the state of a Controller at a certain point.
type Condition struct {
	// Type of condition.
	Type string
	// Status of the condition, one of True, False, Unknown.
	Status string
	// The reason for the condition's last transition.
	Reason string
	// A human readable message indicating details about the transition.
	Message string
}

// Controller describes a cluster resource that declares versioned images.
type Controller struct {
	ID        flux.ResourceID
	Status    string // A status summary for display
	PodStatus ControllerPodStatus
	// Is the controller considered read-only because it's under the
	// control of the platform. In the case of Kubernetes, we simply
	// omit these controllers; but this may not always be the case.
	IsSystem bool
	// If this workload was created _because_ of another, antecedent
	// resource through some mechanism (like an operator, or custom
	// resource controller), we try to record the ID of that resource
	// in this field.
	Antecedent flux.ResourceID
	Labels     map[string]string

	Containers ContainersOrExcuse
}

// Sometimes we care if we can't find the containers for a service,
// sometimes we just want the information we can get.
type ContainersOrExcuse struct {
	Excuse     string
	Containers []resource.Container
}

func (s Controller) ContainersOrNil() []resource.Container {
	return s.Containers.Containers
}

func (s Controller) ContainersOrError() ([]resource.Container, error) {
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
