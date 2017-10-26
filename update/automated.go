package update

import (
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
)

type Automated struct {
	Changes []Change
}

type Change struct {
	ServiceID flux.ResourceID
	Container cluster.Container
	ImageID   flux.ImageRef
}

func (a *Automated) Add(service flux.ResourceID, container cluster.Container, image flux.ImageRef) {
	a.Changes = append(a.Changes, Change{service, container, image})
}

func (a *Automated) CalculateRelease(rc ReleaseContext, logger log.Logger) ([]*ControllerUpdate, Result, error) {
	filters, err := a.filters(rc)
	if err != nil {
		return nil, nil, err
	}

	result := Result{}
	updates, err := rc.SelectServices(result, filters...)
	if err != nil {
		return nil, nil, err
	}

	a.markSkipped(result)
	updates, err = a.calculateImageUpdates(rc, updates, result, logger)
	if err != nil {
		return nil, nil, err
	}

	return updates, result, err
}

func (a *Automated) ReleaseType() ReleaseType {
	return "automated"
}

func (a *Automated) ReleaseKind() ReleaseKind {
	return ReleaseKindExecute
}

func (a *Automated) CommitMessage() string {
	var images []string
	for _, image := range a.Images() {
		images = append(images, image.String())
	}
	return fmt.Sprintf("Release %s to automated", strings.Join(images, ", "))
}

func (a *Automated) Images() []flux.ImageRef {
	imageMap := map[flux.ImageRef]struct{}{}
	for _, change := range a.Changes {
		imageMap[change.ImageID] = struct{}{}
	}
	var images []flux.ImageRef
	for image, _ := range imageMap {
		images = append(images, image)
	}
	return images
}

func (a *Automated) filters(rc ReleaseContext) ([]ControllerFilter, error) {
	return []ControllerFilter{
		&IncludeFilter{a.serviceIDs()},
	}, nil
}

func (a *Automated) markSkipped(results Result) {
	for _, v := range a.serviceIDs() {
		if _, ok := results[v]; !ok {
			results[v] = ControllerResult{
				Status: ReleaseStatusSkipped,
				Error:  NotInRepo,
			}
		}
	}
}

func (a *Automated) calculateImageUpdates(rc ReleaseContext, candidates []*ControllerUpdate, result Result, logger log.Logger) ([]*ControllerUpdate, error) {
	updates := []*ControllerUpdate{}

	serviceMap := a.serviceMap()
	for _, u := range candidates {
		containers, err := u.Controller.ContainersOrError()
		if err != nil {
			result[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusFailed,
				Error:  err.Error(),
			}
			continue
		}

		changes := serviceMap[u.ResourceID]
		containerUpdates := []ContainerUpdate{}
		for _, container := range containers {
			currentImageID, err := flux.ParseImageRef(container.Image)
			if err != nil {
				return nil, err
			}

			for _, change := range changes {
				if change.Container.Name != container.Name {
					continue
				}

				u.ManifestBytes, err = rc.Manifests().UpdateDefinition(u.ManifestBytes, container.Name, change.ImageID)
				if err != nil {
					return nil, err
				}

				containerUpdates = append(containerUpdates, ContainerUpdate{
					Container: container.Name,
					Current:   currentImageID,
					Target:    change.ImageID,
				})
			}
		}

		if len(containerUpdates) > 0 {
			u.Updates = containerUpdates
			updates = append(updates, u)
			result[u.ResourceID] = ControllerResult{
				Status:       ReleaseStatusSuccess,
				PerContainer: containerUpdates,
			}
		} else {
			result[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusIgnored,
				Error:  DoesNotUseImage,
			}
		}
	}

	return updates, nil
}

func (a *Automated) serviceMap() map[flux.ResourceID][]Change {
	set := map[flux.ResourceID][]Change{}
	for _, change := range a.Changes {
		set[change.ServiceID] = append(set[change.ServiceID], change)
	}
	return set
}

func (a *Automated) serviceIDs() []flux.ResourceID {
	slice := []flux.ResourceID{}
	for service, _ := range a.serviceMap() {
		slice = append(slice, flux.MustParseResourceID(service.String()))
	}
	return slice
}
