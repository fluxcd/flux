package flux

import (
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

type Helper struct {
	Platform *kubernetes.Cluster
	Registry *registry.Client
	Logger   log.Logger
}

func (s *Helper) AllServices() ([]ServiceID, error) {
	namespaces, err := s.Platform.Namespaces()
	if err != nil {
		return nil, errors.Wrap(err, "fetching platform namespaces")
	}

	var res []ServiceID
	for _, namespace := range namespaces {
		services, err := s.Platform.Services(namespace)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching platform services for namespace %q", namespace)
		}

		for _, service := range services {
			id := MakeServiceID(namespace, service.Name)
			res = append(res, id)
		}
	}

	return res, nil
}

func (s *Helper) AllImagesFor(serviceIDs []ServiceID) (map[ServiceID][]platform.Container, error) {
	containerMap := map[ServiceID][]platform.Container{}
	for _, serviceID := range serviceIDs {
		namespace, service := serviceID.Components()
		containers, err := s.Platform.ContainersFor(namespace, service)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching containers for %s", serviceID)
		}
		if len(containers) <= 0 {
			continue
		}
		containerMap[serviceID] = containers
	}
	return containerMap, nil
}

func (s *Helper) Log(args ...interface{}) {
	s.Logger.Log(args...)
}
