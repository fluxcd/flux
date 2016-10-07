package instance

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

type Instancer interface {
	Get(inst flux.InstanceID) (*Instance, error)
}

type Instance struct {
	platform *kubernetes.Cluster
	registry *registry.Client
	config   Configurer
	duration metrics.Histogram
	log.Logger
	history.EventReader
	history.EventWriter
}

func New(
	platform *kubernetes.Cluster,
	registry *registry.Client,
	config Configurer,
	logger log.Logger,
	duration metrics.Histogram,
	events history.EventReader,
	eventlog history.EventWriter,
) *Instance {
	return &Instance{
		platform:    platform,
		registry:    registry,
		config:      config,
		duration:    duration,
		Logger:      logger,
		EventReader: events,
		EventWriter: eventlog,
	}
}

func (h *Instance) AllServices() (res []flux.ServiceID, err error) {
	defer func(begin time.Time) {
		h.duration.With(
			"method", "AllServices",
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	namespaces, err := h.platform.Namespaces()
	if err != nil {
		return nil, errors.Wrap(err, "fetching platform namespaces")
	}

	for _, namespace := range namespaces {
		ids, err := h.NamespaceServices(namespace)
		if err != nil {
			return nil, err
		}
		res = append(res, ids...)
	}

	return res, nil
}

func (h *Instance) NamespaceServices(namespace string) (res []flux.ServiceID, err error) {
	defer func(begin time.Time) {
		h.duration.With(
			"method", "NamespaceServices",
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	services, err := h.platform.Services(namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching platform services for namespace %q", namespace)
	}

	res = make([]flux.ServiceID, len(services))
	for i, service := range services {
		res[i] = flux.MakeServiceID(namespace, service.Name)
	}

	return res, nil
}

// AllReleasableImagesFor returns a map of service IDs to the
// containers with images that may be regraded. It leaves out any
// services that cannot have containers associated with them, e.g.,
// because there is no matching deployment.
func (h *Instance) AllReleasableImagesFor(serviceIDs []flux.ServiceID) (res map[flux.ServiceID][]platform.Container, err error) {
	defer func(begin time.Time) {
		h.duration.With(
			"method", "AllReleasableImagesFor",
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	res = map[flux.ServiceID][]platform.Container{}
	for _, serviceID := range serviceIDs {
		namespace, service := serviceID.Components()
		containers, err := h.platform.ContainersFor(namespace, service)
		if err != nil {
			switch err {
			case platform.ErrEmptySelector, platform.ErrServiceHasNoSelector, platform.ErrNoMatching, platform.ErrMultipleMatching, platform.ErrNoMatchingImages:
				continue
			default:
				return nil, errors.Wrapf(err, "fetching containers for %s", serviceID)
			}
		}
		if len(containers) <= 0 {
			continue
		}
		res[serviceID] = containers
	}
	return res, nil
}

func (h *Instance) PlatformService(namespace, service string) (res platform.Service, err error) {
	defer func(begin time.Time) {
		h.duration.With(
			"method", "PlatformService",
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return h.platform.Service(namespace, service)
}

func (h *Instance) PlatformNamespaces() (res []string, err error) {
	defer func(begin time.Time) {
		h.duration.With(
			"method", "PlatformNamespaces",
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return h.platform.Namespaces()
}

func (h *Instance) PlatformContainersFor(namespace, service string) (res []platform.Container, err error) {
	defer func(begin time.Time) {
		h.duration.With(
			"method", "PlatformContainersFor",
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return h.platform.ContainersFor(namespace, service)
}

func (h *Instance) RegistryGetRepository(repository string) (res *registry.Repository, err error) {
	defer func(begin time.Time) {
		h.duration.With(
			"method", "RegistryGetRepository",
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return h.registry.GetRepository(repository)
}

func (h *Instance) PlatformRegrade(specs []platform.RegradeSpec) (err error) {
	defer func(begin time.Time) {
		h.duration.With(
			"method", "PlatformRegrade",
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return h.platform.Regrade(specs)
}

func (h *Instance) GetConfig() (Config, error) {
	return h.config.Get()
}

func (h *Instance) UpdateConfig(update UpdateFunc) error {
	return h.config.Update(update)
}
