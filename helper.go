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
		ids, err := s.NamespaceServices(namespace)
		if err != nil {
			return nil, err
		}
		res = append(res, ids...)
	}

	return res, nil
}

func (s *Helper) NamespaceServices(namespace string) ([]ServiceID, error) {
	services, err := s.Platform.Services(namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching platform services for namespace %q", namespace)
	}

	res := make([]ServiceID, len(services))
	for i, service := range services {
		res[i] = MakeServiceID(namespace, service.Name)
	}

	return res, nil
}

// AllReleasableImagesFor returns a map of service IDs to the
// containers with images that may be regraded. It leaves out any
// services that cannot have containers associated with them, e.g.,
// because there is no matching deployment.
func (s *Helper) AllReleasableImagesFor(serviceIDs []ServiceID) (map[ServiceID][]platform.Container, error) {
	containerMap := map[ServiceID][]platform.Container{}
	for _, serviceID := range serviceIDs {
		namespace, service := serviceID.Components()
		containers, err := s.Platform.ContainersFor(namespace, service)
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
		containerMap[serviceID] = containers
	}
	return containerMap, nil
}

func (s *Helper) Log(args ...interface{}) {
	s.Logger.Log(args...)
}
