package cluster

import (
	"errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
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
	// Update the definitions in the manifest file according to the
	// spec given.
	UpdateDefinition(def []byte, newImageID flux.ImageID) ([]byte, error)
	// Load all the resource manifests under the path given
	LoadManifests(path string) (map[string]resource.Resource, error)
	// Parse the manifests given in an exported blob
	ParseManifests([]byte) (map[string]resource.Resource, error)
	// UpdateManifest applies a resource modifying function to the definition,
	// and writes the result back.
	UpdateManifest(path, resourceID string, f func(def []byte) ([]byte, error)) error
	// UpdatePolicies modifies a manifest to apply the policy update specified
	UpdatePolicies([]byte, policy.Update) ([]byte, error)
	// ServicesWithPolicy finds the services which have a particular policy set on them.
	ServicesWithPolicy(path string, p policy.Policy) (flux.ServiceIDSet, error)
}

// Service describes a platform service, generally a floating IP with one or
// more exposed ports that map to a load-balanced pool of instances. Eventually
// this type will generalize to something of a lowest-common-denominator for
// all supported platforms, but right now it looks a lot like a Kubernetes
// service.
type Service struct {
	ID       flux.ServiceID
	IP       string
	Metadata map[string]string // a grab bag of goodies, likely platform-specific
	Status   string            // A status summary for display

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

func (s Service) ContainersOrNil() []Container {
	return s.Containers.Containers
}

func (s Service) ContainersOrError() ([]Container, error) {
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
