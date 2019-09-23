package update

import (
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/resource"
)

const (
	Locked                 = "locked"
	Ignore                 = "ignore"
	NotIncluded            = "not included"
	Excluded               = "excluded"
	DifferentImage         = "a different image"
	NotAccessibleInCluster = "not accessible in cluster"
	NotInRepo              = "not found in repository"
	ImageNotFound          = "cannot find one or more images"
	ImageUpToDate          = "image(s) up to date"
	DoesNotUseImage        = "does not use image(s)"
	ContainerNotFound      = "container(s) not found: %s"
	ContainerTagMismatch   = "container(s) tag mismatch: %s"
)

type SpecificImageFilter struct {
	Img image.Ref
}

func (f *SpecificImageFilter) Filter(u WorkloadUpdate) WorkloadResult {
	// If there are no containers, then we can't check the image.
	if len(u.Workload.Containers.Containers) == 0 {
		return WorkloadResult{
			Status: ReleaseStatusIgnored,
			Error:  NotAccessibleInCluster,
		}
	}
	// For each container in update
	for _, c := range u.Workload.Containers.Containers {
		if c.Image.CanonicalName() == f.Img.CanonicalName() {
			// We want to update this
			return WorkloadResult{}
		}
	}
	return WorkloadResult{
		Status: ReleaseStatusIgnored,
		Error:  DifferentImage,
	}
}

type ExcludeFilter struct {
	IDs []resource.ID
}

func (f *ExcludeFilter) Filter(u WorkloadUpdate) WorkloadResult {
	for _, id := range f.IDs {
		if u.ResourceID == id {
			return WorkloadResult{
				Status: ReleaseStatusIgnored,
				Error:  Excluded,
			}
		}
	}
	return WorkloadResult{}
}

type IncludeFilter struct {
	IDs []resource.ID
}

func (f *IncludeFilter) Filter(u WorkloadUpdate) WorkloadResult {
	for _, id := range f.IDs {
		if u.ResourceID == id {
			return WorkloadResult{}
		}
	}
	return WorkloadResult{
		Status: ReleaseStatusIgnored,
		Error:  NotIncluded,
	}
}

type LockedFilter struct {
}

func (f *LockedFilter) Filter(u WorkloadUpdate) WorkloadResult {
	if u.Resource.Policies().Has(policy.Locked) {
		return WorkloadResult{
			Status: ReleaseStatusSkipped,
			Error:  Locked,
		}
	}
	return WorkloadResult{}
}

type IgnoreFilter struct {
}

func (f *IgnoreFilter) Filter(u WorkloadUpdate) WorkloadResult {
	if u.Workload.Policies.Has(policy.Ignore) {
		return WorkloadResult{
			Status: ReleaseStatusSkipped,
			Error:  Ignore,
		}
	}
	return WorkloadResult{}
}
