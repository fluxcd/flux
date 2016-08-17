package flux

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy/automator"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

type server struct {
	platform  *kubernetes.Cluster
	registry  *registry.Client
	automator *automator.Automator
	history   history.DB
}

func NewServer(platform *kubernetes.Cluster, registry *registry.Client, automator *automator.Automator) Service {
	return &server{
		platform:  platform,
		registry:  registry,
		automator: automator,
	}
}

// The server methods are deliberately awkward, cobbled together from existing
// platform and registry APIs. I want to avoid changing those components until I
// get something working. There's also a lot of code duplication here for the
// same reason: let's not add abstraction until it's merged, or nearly so, and
// it's clear where the abstraction should exist.

func (s *server) ListServices() ([]ServiceDescription, error) {
	var res []ServiceDescription
	for _, namespace := range []string{
		"default", // TODO(pb): s.platform.Namespaces()
	} {
		services, err := s.platform.Services(namespace)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching platform services for namespace %q", namespace)
		}

		for _, service := range services {
			idStr := fmt.Sprintf("%s/%s", namespace, service.Name)
			id, err := ParseServiceID(idStr)
			if err != nil {
				return nil, errors.Wrapf(err, "parsing service ID %q", idStr)
			}

			// TODO(pb): containers should be returned as part of Services
			containers, err := s.platform.ContainersFor(namespace, service.Name)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching containers for %s/%s", namespace, service.Name)
			}

			var c []Container
			for _, container := range containers {
				imageID := ParseImageID(container.Image)
				imageRepo, err := s.registry.GetRepository(imageID.Repository())
				if err != nil {
					return nil, errors.Wrapf(err, "fetching image repo for %s", imageID)
				}

				var (
					current   ImageDescription
					available []ImageDescription
				)
				for _, image := range imageRepo.Images {
					description := ImageDescription{
						ID:        ParseImageID(image.String()),
						CreatedAt: image.CreatedAt,
					}
					available = append(available, description)
					if image.String() == container.Image {
						current = description
					}
				}
				c = append(c, Container{
					Name:      container.Name,
					Current:   current,
					Available: available,
				})
			}

			res = append(res, ServiceDescription{
				ID:         id,
				Containers: c,
			})
		}
	}

	return res, nil
}

func (s *server) ListImages(spec ServiceSpec) ([]ImageDescription, error) {
	m := map[string][]platform.Service{}
	if spec == ServiceSpecAll {
		for _, namespace := range []string{
			"default", // TODO(pb): s.platform.Namespaces()
		} {
			services, err := s.platform.Services(namespace)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching platform services for namespace %q", namespace)
			}

			m[namespace] = services
		}
	} else {
		id, err := ParseServiceID(string(spec))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid service spec %s", spec)
		}

		namespace, service := id.Components()
		svc, err := s.platform.Service(namespace, service)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching platform service %s", id)
		}

		m[namespace] = []platform.Service{svc}
	}

	var res []ImageDescription
	for namespace, services := range m {
		for _, service := range services {
			containers, err := s.platform.ContainersFor(namespace, service.Name)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching containers for %s/%s", namespace, service.Name)
			}

			for _, container := range containers {
				imageID := ParseImageID(container.Image)
				imageRepo, err := s.registry.GetRepository(imageID.Repository())
				if err != nil {
					return nil, errors.Wrapf(err, "fetching image repo for %s", imageID)
				}

				for _, image := range imageRepo.Images {
					res = append(res, ImageDescription{
						ID:        ParseImageID(image.String()),
						CreatedAt: image.CreatedAt,
					})
				}
			}
		}
	}

	return res, nil
}

func (s *server) Release(serviceSpec ServiceSpec, imageSpec ImageSpec, kind ReleaseKind) ([]ReleaseAction, error) {
	return nil, errors.New("Release not implemented by server")
}

func (s *server) Automate(id ServiceID) error {
	namespace, service := id.Components()
	s.automator.Enable(namespace, service)
	return nil
}

func (s *server) Deautomate(id ServiceID) error {
	namespace, service := id.Components()
	s.automator.Disable(namespace, service)
	return nil
}

func (s *server) History(spec ServiceSpec) ([]HistoryEntry, error) {
	var events []history.Event
	if spec == ServiceSpecAll {
		for _, namespace := range []string{
			"default", // TODO(pb)
		} {
			ev, err := s.history.AllEvents(namespace)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching all history events for namespace %s", namespace)
			}
			events = append(events, ev...)
		}
	} else {
		id, err := ParseServiceID(string(spec))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid service spec %s", spec)
		}

		namespace, service := id.Components()
		ev, err := s.history.EventsForService(namespace, service)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching history events for %s", id)
		}

		events = append(events, ev...)
	}

	res := make([]HistoryEntry, len(events))
	for i, event := range events {
		res[i] = HistoryEntry{
			Stamp: event.Stamp,
			Type:  "default",
			Data:  fmt.Sprintf("%s: %s", event.Service, event.Msg),
		}
	}

	return res, nil
}
