package flux

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

type server struct {
	platform *kubernetes.Cluster
	registry *registry.Client
}

func NewServer(platform *kubernetes.Cluster, registry *registry.Client) Service {
	return &server{
		platform: platform,
		registry: registry,
	}
}

func (s *server) ListServices() ([]ServiceDescription, error) {
	// This function is deliberately awkward, cobbled together from existing
	// platform and registry APIs. I wanted to avoid changing those components
	// until I got something working.

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

func (s *server) ListImages(ServiceSpec) ([]ImageDescription, error) {
	return nil, errors.New("ListImages not implemented by server")
}

func (s *server) Release(ServiceSpec, ImageSpec, ReleaseKind) ([]ReleaseAction, error) {
	return nil, errors.New("Release not implemented by server")
}

func (s *server) Automate(ServiceID) error {
	return errors.New("Automate not implemented by server")
}

func (s *server) Deautomate(ServiceID) error {
	return errors.New("Deautomate not implemented by server")
}

func (s *server) History(ServiceSpec) ([]HistoryEntry, error) {
	return nil, errors.New("History not implemented by server")
}
