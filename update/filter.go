package update

import "github.com/weaveworks/flux"

const (
	Locked          = "locked"
	NotIncluded     = "not included"
	Excluded        = "excluded"
	DifferentImage  = "a different image"
	NotInCluster    = "not running in cluster"
	NotInRepo       = "not found in repository"
	ImageNotFound   = "cannot find one or more images"
	ImageUpToDate   = "image(s) up to date"
	DoesNotUseImage = "does not use image(s)"
)

type SpecificImageFilter struct {
	Img flux.ImageID
}

func (f *SpecificImageFilter) Filter(u ServiceUpdate) ServiceResult {
	// If there are no containers, then we can't check the image.
	if len(u.Service.Containers.Containers) == 0 {
		return ServiceResult{
			Status: ReleaseStatusIgnored,
			Error:  NotInCluster,
		}
	}
	// For each container in update
	for _, c := range u.Service.Containers.Containers {
		cID, _ := flux.ParseImageID(c.Image)
		// If container image == image in update
		if cID.HostImage() == f.Img.HostImage() {
			// We want to update this
			return ServiceResult{}
		}
	}
	return ServiceResult{
		Status: ReleaseStatusIgnored,
		Error:  DifferentImage,
	}
}

type ExcludeFilter struct {
	IDs []flux.ResourceID
}

func (f *ExcludeFilter) Filter(u ServiceUpdate) ServiceResult {
	for _, id := range f.IDs {
		if u.ServiceID == id {
			return ServiceResult{
				Status: ReleaseStatusIgnored,
				Error:  Excluded,
			}
		}
	}
	return ServiceResult{}
}

type IncludeFilter struct {
	IDs []flux.ResourceID
}

func (f *IncludeFilter) Filter(u ServiceUpdate) ServiceResult {
	for _, id := range f.IDs {
		if u.ServiceID == id {
			return ServiceResult{}
		}
	}
	return ServiceResult{
		Status: ReleaseStatusIgnored,
		Error:  NotIncluded,
	}
}

type LockedFilter struct {
	IDs []flux.ResourceID
}

func (f *LockedFilter) Filter(u ServiceUpdate) ServiceResult {
	for _, id := range f.IDs {
		if u.ServiceID == id {
			return ServiceResult{
				Status: ReleaseStatusSkipped,
				Error:  Locked,
			}
		}
	}
	return ServiceResult{}
}
