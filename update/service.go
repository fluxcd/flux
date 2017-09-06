package update

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
)

type ServiceUpdate struct {
	ServiceID     flux.ResourceID
	Service       cluster.Service
	ManifestPath  string
	ManifestBytes []byte
	Updates       []ContainerUpdate
}

type ServiceFilter interface {
	Filter(ServiceUpdate) ServiceResult
}

func (s *ServiceUpdate) Filter(filters ...ServiceFilter) ServiceResult {
	for _, f := range filters {
		fr := f.Filter(*s)
		if fr.Error != "" {
			return fr
		}
	}
	return ServiceResult{}
}
