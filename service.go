package flux

import (
	"errors"
	"time"

	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

// Service is the flux.Service, i.e. what is implemented by fluxd.
// It deals in (among other things) services on the platform.
type Service interface {
	// Images returns the images that are available in a repository.
	// Always in reverse chronological order, i.e. newest first.
	Images(repository string) ([]registry.Image, error)

	// Services returns the currently active services on the platform.
	Services(namespace string) ([]platform.Service, error)

	// Release migrates a service from its current image to a new image, derived
	// from the newDef definition. Right now, that needs to be the body of a
	// replication controller. A rolling-update is performed with the provided
	// updatePeriod. This call blocks until it's complete.
	Release(namespace, service string, newDef []byte, updatePeriod time.Duration) error
}

var (
	// ErrNoPlatformConfigured indicates a service was constructed without a
	// reference to a runtime platform. A programmer or configuration error.
	ErrNoPlatformConfigured = errors.New("no platform configured")
)

// NewService returns a service connected to the provided Kubernetes platform.
func NewService(reg *registry.Client, k8s *kubernetes.Cluster) Service {
	return &service{
		registry: reg,
		platform: k8s,
	}
}

type service struct {
	registry *registry.Client
	platform *kubernetes.Cluster // TODO(pb): replace with platform.Platform when we have that
}

func (s *service) Images(repository string) ([]registry.Image, error) {
	repo, err := s.registry.GetRepository(repository)
	if err != nil {
		return nil, err
	}
	return repo.Images, nil
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
