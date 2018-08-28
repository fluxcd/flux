package update

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
)

const (
	Locked               = "locked"
	NotIncluded          = "not included"
	Excluded             = "excluded"
	DifferentImage       = "a different image"
	NotInCluster         = "not running in cluster"
	NotInRepo            = "not found in repository"
	ImageNotFound        = "cannot find one or more images"
	ImageUpToDate        = "image(s) up to date"
	DoesNotUseImage      = "does not use image(s)"
	ContainerNotFound    = "container(s) not found: %s"
	ContainerTagMismatch = "container(s) tag mismatch: %s"
)

type SpecificImageFilter struct {
	Img image.Ref
}

func (f *SpecificImageFilter) Filter(u ControllerUpdate) ControllerResult {
	// If there are no containers, then we can't check the image.
	if len(u.Controller.Containers.Containers) == 0 {
		return ControllerResult{
			Status: ReleaseStatusIgnored,
			Error:  NotInCluster,
		}
	}
	// For each container in update
	for _, c := range u.Controller.Containers.Containers {
		if c.Image.CanonicalName() == f.Img.CanonicalName() {
			// We want to update this
			return ControllerResult{}
		}
	}
	return ControllerResult{
		Status: ReleaseStatusIgnored,
		Error:  DifferentImage,
	}
}

type ExcludeFilter struct {
	IDs []flux.ResourceID
}

func (f *ExcludeFilter) Filter(u ControllerUpdate) ControllerResult {
	for _, id := range f.IDs {
		if u.ResourceID == id {
			return ControllerResult{
				Status: ReleaseStatusIgnored,
				Error:  Excluded,
			}
		}
	}
	return ControllerResult{}
}

type IncludeFilter struct {
	IDs []flux.ResourceID
}

func (f *IncludeFilter) Filter(u ControllerUpdate) ControllerResult {
	for _, id := range f.IDs {
		if u.ResourceID == id {
			return ControllerResult{}
		}
	}
	return ControllerResult{
		Status: ReleaseStatusIgnored,
		Error:  NotIncluded,
	}
}

type LockedFilter struct {
}

func (f *LockedFilter) Filter(u ControllerUpdate) ControllerResult {
	if u.Resource.Policy().Has(policy.Locked) {
		return ControllerResult{
			Status: ReleaseStatusSkipped,
			Error:  Locked,
		}
	}
	return ControllerResult{}
}
