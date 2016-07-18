package flux

import (
	"errors"
	"strings"
	"time"

	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

const DefaultNamespace = "default"

// Service is the flux.Service, i.e. what is implemented by fluxd.
// It deals in (among other things) services on the platform.
type Service interface {
	// Images returns the images that are available in a repository.
	// Always in reverse chronological order, i.e. newest first.
	Images(repository string) ([]registry.Image, error)

	// ServiceImages returns a list of (container, images),
	// representing the running state (the container) along with the
	// potentially releasable state (the images)
	ServiceImages(namespace, service string) ([]ContainerImages, error)

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

type ContainerImages struct {
	Container platform.Container
	Images    []registry.Image
}

func (s *service) Images(repository string) ([]registry.Image, error) {
	repo, err := s.registry.GetRepository(repository)
	if err != nil {
		return nil, err
	}
	return repo.Images, nil
}

func (s *service) ServiceImages(namespace, service string) ([]ContainerImages, error) {
	containers, err := s.platform.ContainersFor(namespace, service)
	if err != nil {
		return nil, err
	}
	result := make([]ContainerImages, 0)
	for _, container := range containers {
		imageParts := strings.SplitN(container.Image, ":", 2)
		repository, err := s.registry.GetRepository(imageParts[0])
		if err != nil {
			return nil, err
		}
		result = append(result, ContainerImages{container, repository.Images})
	}
	return result, nil
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
