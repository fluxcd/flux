package flux

import (
	"errors"
	"time"

	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/kubernetes"
)

// Service is the flux.Service, i.e. what is implemented by fluxd.
// It deals in (among other things) services on the platform.
type Service interface {
	// Services returns the currently active services on the platform.
	Services(namespace string) ([]platform.Service, error)

	// Release migrates a service from its current image to a new image, derived
	// from the newDef definition. Right now, that needs to be the body of a
	// replication controller. A rolling-update is performed with the provided
	// updatePeriod. This call blocks until it's complete.
	Release(namespace, service string, newDef []byte, updatePeriod time.Duration) error
}

// NewService returns a service connected to the provided Kubernetes platform.
func NewService(k8s *kubernetes.Cluster) Service {
	return &service{
		platform: k8s,
	}
}

var (
	// ErrNoPlatformConfigured indicates a service was constructed without a
	// reference to a runtime platform. A programmer or configuration error.
	ErrNoPlatformConfigured = errors.New("no platform configured")
)

type service struct {
	platform *kubernetes.Cluster // TODO(pb): replace with platform.Platform when we have that
}

func (s *service) Services(namespace string) ([]platform.Service, error) {
	if s.platform == nil {
		return nil, ErrNoPlatformConfigured
	}
	return s.platform.Services(namespace)
}

func (s *service) Release(namespace, service string, newDef []byte, updatePeriod time.Duration) error {
	if s.platform == nil {
		return ErrNoPlatformConfigured
	}
	return s.platform.Release(namespace, service, newDef, updatePeriod)
}
