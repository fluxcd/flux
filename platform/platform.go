// Package platform will hold abstractions and data types common to supported
// platforms. We don't know what all of those will look like, yet. So the
// package is mostly empty.
package platform

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
)

var (
	ErrPlatformNotAvailable = errors.New("Platform is not available")
)

// Platform is the interface various platforms fulfill, e.g.
// *kubernetes.Cluster
type Platform interface {
	AllServices(maybeNamespace string, ignored flux.ServiceIDSet) ([]Service, error)
	SomeServices([]flux.ServiceID) ([]Service, error)
	Regrade([]RegradeSpec) error
}

// For getting a connection to a platform; this can happen in
// different ways, e.g., by having direct access to Kubernetes in
// standalone mode, or by going via a message bus.
type Connecter interface {
	Connect(inst flux.InstanceID) (Platform, error)
}

// MessageBus handles routing messages to/from the matching platform.
type MessageBus interface {
	Connecter
	Subscribe(inst flux.InstanceID, p Platform) error
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

// These errors all represent logical problems with platform
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

// RegradeSpec is provided to platform.Regrade method/s.
type RegradeSpec struct {
	ServiceID     flux.ServiceID
	NewDefinition []byte // of the pod controller e.g. deployment
}

type RegradeError map[flux.ServiceID]error

func (e RegradeError) Error() string {
	var errs []string
	for id, err := range e {
		errs = append(errs, fmt.Sprintf("%s: %v", id, err))
	}
	return strings.Join(errs, "; ")
}
