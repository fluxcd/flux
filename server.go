package flux

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

type server struct {
	helper    Helper
	releaser  Releaser
	automator Automator
	history   history.DB
}

type Automator interface {
	Automate(namespace, service string) error
	Deautomate(namespace, service string) error
	IsAutomated(namespace, service string) bool
}

type Releaser interface {
	Release(ServiceSpec, ImageSpec, ReleaseKind) ([]ReleaseAction, error)
}

func NewServer(platform *kubernetes.Cluster, registry *registry.Client, releaser Releaser, automator Automator, history history.DB, logger log.Logger) Service {
	return &server{
		helper: Helper{
			Platform: platform,
			Registry: registry,
			Logger:   logger,
		},
		releaser:  releaser,
		automator: automator,
		history:   history,
	}
}

// The server methods are deliberately awkward, cobbled together from existing
// platform and registry APIs. I want to avoid changing those components until I
// get something working. There's also a lot of code duplication here for the
// same reason: let's not add abstraction until it's merged, or nearly so, and
// it's clear where the abstraction should exist.

func (s *server) ListServices() (res []ServiceStatus, err error) {
	s.helper.Log("method", "ListServices")
	defer func() { s.helper.Log("method", "ListServices", "res", len(res), "err", err) }()

	serviceIDs, err := s.helper.AllServices()
	if err != nil {
		return nil, errors.Wrap(err, "fetching all services on the platform")
	}

	for _, serviceID := range serviceIDs {
		namespace, service := serviceID.Components()

		// TODO(pb): containers should be returned as part of Services
		var c []Container
		containers, err := s.helper.Platform.ContainersFor(namespace, service)
		if err != nil {
			s.helper.Log("err", errors.Wrapf(err, "fetching containers for %s", serviceID))
		} else {
			for _, container := range containers {
				imageID := ParseImageID(container.Image)
				imageRepo, err := s.helper.Registry.GetRepository(imageID.Repository())
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
		}

		// FIXME: since we get service IDs above, we have to get the
		// service; but this is repeating work, since to get the
		// service IDs we got a list of all the services ...
		platformSvc, err := s.helper.Platform.Service(namespace, service)
		if err != nil {
			return nil, errors.Wrapf(err, "getting platform service %s", serviceID)
		}

		res = append(res, ServiceStatus{
			ID:         serviceID,
			Containers: c,
			Status:     platformSvc.Status,
			Automated:  s.automator.IsAutomated(namespace, service),
		})
	}

	return res, nil
}

func (s *server) ListImages(spec ServiceSpec) (res []ImageStatus, err error) {
	s.helper.Log("method", "ListImages", "spec", spec)
	defer func() { s.helper.Log("method", "ListImages", "spec", spec, "res", len(res), "err", err) }()

	serviceIDs, err := func() ([]ServiceID, error) {
		if spec == ServiceSpecAll {
			return s.helper.AllServices()
		}
		id, err := ParseServiceID(string(spec))
		return []ServiceID{id}, err
	}()
	if err != nil {
		return nil, errors.Wrapf(err, "fetching service ID(s)")
	}

	for _, serviceID := range serviceIDs {
		namespace, service := serviceID.Components()

		var c []Container
		containers, err := s.helper.Platform.ContainersFor(namespace, service)
		if err != nil {
			s.helper.Log("err", errors.Wrapf(err, "fetching containers for %s", serviceID))
		} else {
			for _, container := range containers {
				imageID := ParseImageID(container.Image)
				imageRepo, err := s.helper.Registry.GetRepository(imageID.Repository())
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
		}

		res = append(res, ImageStatus{
			ID:         serviceID,
			Containers: c,
		})
	}

	return res, nil
}

func (s *server) History(spec ServiceSpec) ([]HistoryEntry, error) {
	var events []history.Event
	if spec == ServiceSpecAll {
		namespaces, err := s.helper.Platform.Namespaces()
		if err != nil {
			return nil, errors.Wrap(err, "fetching platform namespaces")
		}

		for _, namespace := range namespaces {
			ev, err := s.history.AllEvents(namespace)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching all history events for namespace %s", namespace)
			}

			events = append(events, ev...)
		}
	} else {
		id, err := ParseServiceID(string(spec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", spec)
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
			Type:  "v0",
			Data:  fmt.Sprintf("%s: %s", event.Service, event.Msg),
		}
	}

	return res, nil
}

func (s *server) Automate(service ServiceID) error {
	ns, svc := service.Components()
	return s.automator.Automate(ns, svc)
}

func (s *server) Deautomate(service ServiceID) error {
	ns, svc := service.Components()
	return s.automator.Deautomate(ns, svc)
}

func (s *server) Release(service ServiceSpec, image ImageSpec, kind ReleaseKind) ([]ReleaseAction, error) {
	return s.releaser.Release(service, image, kind)
}
