package update

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
)

type ControllerUpdate struct {
	ResourceID    flux.ResourceID
	Controller    cluster.Controller
	ManifestPath  string
	ManifestBytes []byte
	Updates       []ContainerUpdate
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
