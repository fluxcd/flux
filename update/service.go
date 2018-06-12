package update

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/resource"
)

type ControllerUpdate struct {
	ResourceID   flux.ResourceID
	Controller   cluster.Controller
	Resource     resource.Workload
	ManifestPath string
	Updates      []ContainerUpdate
}

type ControllerFilter interface {
	Filter(ControllerUpdate) ControllerResult
}

func (s *ControllerUpdate) Filter(filters ...ControllerFilter) ControllerResult {
	for _, f := range filters {
		fr := f.Filter(*s)
		if fr.Error != "" {
			return fr
		}
	}
	return ControllerResult{}
}
