package update

import (
	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/resource"
)

type WorkloadUpdate struct {
	ResourceID resource.ID
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
