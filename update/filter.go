package update

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
)

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
		cID, _ := image.ParseRef(c.Image)
		// If container image == image in update
		if cID.CanonicalName() == f.Img.CanonicalName() {
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
	IDs []flux.ResourceID
}

func (f *LockedFilter) Filter(u ControllerUpdate) ControllerResult {
	for _, id := range f.IDs {
		if u.ResourceID == id {
			return ControllerResult{
				Status: ReleaseStatusSkipped,
				Error:  Locked,
			}
		}
	}
	return ControllerResult{}
}

type namespace struct {
	Namespace string
	Kind      string
}

type NamespacesFilter struct {
	namespaces []namespace
}

func (f *NamespacesFilter) Filter(u ControllerUpdate) ControllerResult {
	if len(f.namespaces) == 0 {
		return ControllerResult{}
	}

	ns, kind, _ := u.ResourceID.Components()
	for _, n := range f.namespaces {
		if n.Namespace == ns && n.Kind == kind {
			return ControllerResult{}
		}
	}
	return ControllerResult{
		Status: ReleaseStatusIgnored,
		Error:  NotIncluded,
	}
}

func (f *NamespacesFilter) Add(ns, kind string) {
	f.namespaces = append(f.namespaces, namespace{Namespace: ns, Kind: kind})
}

func (f *NamespacesFilter) Length() int {
	return len(f.namespaces)
}

