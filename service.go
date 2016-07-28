package flux

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/fluxy/automator"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

// DefaultNamespace is used when no namespace is provided to service methods.
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

	// History returns the release history of one or all services
	History(namespace, service string) (map[string]history.History, error)

	// Release migrates a service from its current image to a new image, derived
	// from the newDef definition. Right now, that needs to be the body of a
	// replication controller. A rolling-update is performed with the provided
	// updatePeriod. This call blocks until it's complete.
	Release(namespace, service string, newDef []byte, updatePeriod time.Duration) error

	// Automate turns on automatic releases for the given service.
	// Read the history for the service to check status.
	Automate(namespace, service string) error

	// Deautomate turns off automatic releases for the given service.
	// Read the history for the service to check status.
	Deautomate(namespace, service string) error
}

// How to add a method in 5 (or 12) easy steps.
//
// 1. Add the method in this file, service.go.
//    a. Add the method to the interface, above. (Always return an error.)
//    b. Add the method and implementation to the service struct, below.
//
// 2. Update endpoints.go.
//    a. Add an endpoint to the Endpoints struct.
//    b. Populate it in the MakeServerEndpoints constructor.
//    c. This requires defining an individual MakeFooEndpoint constructor.
//    d. That in turn requires defining fooRequest and fooResponse structs.
//
// 3. Update transport.go.
//    a. Wire up the endpoint in MakeHTTPHandler.
//    b. That requires defining decodeFooRequest and encodeFooResponse funcs.
//
// 4. Update client.go.
//    a. Add a httptransport.NewClient call to the NewClient constructor.
//    b. That requires defining encodeFooRequest and decodeFooResponse funcs,
//       which actually live back in transport.go.
//    c. Add the relevant method to the serviceWrapper struct.
//
// 5. Update middlewares.go.
//    a. Add the relevant method to each middleware defined there.

var (
	// ErrNoPlatformConfigured indicates a service was constructed without a
	// reference to a runtime platform. A programmer or configuration error.
	ErrNoPlatformConfigured = errors.New("no platform configured")
)

// NewService returns a service connected to the provided Kubernetes platform.
func NewService(reg *registry.Client, k8s *kubernetes.Cluster, auto *automator.Automator, history history.DB, logger log.Logger) Service {
	return &service{
		registry:  reg,
		platform:  k8s,
		automator: auto,
		history:   history,
		logger:    logger,
	}
}

type service struct {
	registry  *registry.Client
	platform  *kubernetes.Cluster // TODO(pb): replace with platform.Platform when we have that
	automator *automator.Automator
	history   history.DB
	logger    log.Logger
}

// ContainerImages describes a combination of a platform container spec, and the
// available images in the corresponding registry.
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
	var result []ContainerImages
	for _, container := range containers {
		repository, err := s.registry.GetRepository(registry.ParseImage(container.Image).Repository())
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

func (s *service) History(namespace, service string) (map[string]history.History, error) {
	if service == "" {
		return s.history.AllEvents(namespace)
	}

	h, err := s.history.EventsForService(namespace, service)
	if err == history.ErrNoHistory {
		// TODO(pb): not super happy with this
		h = history.History{
			Service: service,
			State:   history.StateUnknown,
		}
	} else if err != nil {
		return nil, err
	}

	return map[string]history.History{
		h.Service: h,
	}, nil
}

func (s *service) Release(namespace, service string, newDef []byte, updatePeriod time.Duration) error {
	if s.platform == nil {
		return ErrNoPlatformConfigured
	}
	err := s.history.ChangeState(namespace, service, history.StateInProgress)
	if err != nil {
		return err
	}

	err = s.platform.Release(namespace, service, newDef, updatePeriod)
	var event string
	if err != nil {
		event = "Release failed: " + err.Error()
	} else {
		event = "Release succeeded"
	}
	if e := s.history.LogEvent(namespace, service, event); e != nil {
		s.logger.Log("method", "Release", "error", e)
	}

	if e := s.history.ChangeState(namespace, service, history.StateRest); e != nil {
		return fmt.Errorf("release completed but unable to change service state: %s", e)
	}
	return err // that from the release itself
}

func (s *service) Automate(namespace, service string) error {
	s.automator.Enable(namespace, service)
	return nil
}

func (s *service) Deautomate(namespace, service string) error {
	s.automator.Disable(namespace, service)
	return nil
}
