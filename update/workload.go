package update

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/resource"
)

type WorkloadUpdate struct {
	ResourceID flux.ResourceID
	Workload   cluster.Workload
	Resource   resource.Workload
	Updates    []ContainerUpdate
}

type WorkloadFilter interface {
	Filter(WorkloadUpdate) WorkloadResult
}

func (s *WorkloadUpdate) Filter(filters ...WorkloadFilter) WorkloadResult {
	for _, f := range filters {
		fr := f.Filter(*s)
		if fr.Error != "" {
			return fr
		}
	}
	return WorkloadResult{}
}
