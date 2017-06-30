package update

import (
	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
)

type Automated struct {
	changes []change
}

type change struct {
	service   flux.ServiceID
	container cluster.Container
	imageID   flux.ImageID
}

func (a *Automated) Add(service flux.ServiceID, container cluster.Container, image flux.ImageID) {
	a.changes = append(a.changes, change{service, container, image})
}

func (a *Automated) CalculateRelease(rc ReleaseContext, logger log.Logger) ([]*ServiceUpdate, Result, error) {
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
	return "(TODO) automated release commit message"
}

func (a *Automated) filters(rc ReleaseContext) ([]ServiceFilter, error) {
	lockedSet, err := rc.ServicesWithPolicy(policy.Locked)
	if err != nil {
		return nil, err
	}

	return []ServiceFilter{
		&IncludeFilter{a.serviceIDs()},
		&LockedFilter{lockedSet.ToSlice()},
	}, nil
}

func (a *Automated) markSkipped(results Result) {
	for _, v := range a.serviceIDs() {
		if _, ok := results[v]; !ok {
			results[v] = ServiceResult{
				Status: ReleaseStatusSkipped,
				Error:  NotInRepo,
			}
		}
	}
}

func (a *Automated) calculateImageUpdates(rc ReleaseContext, candidates []*ServiceUpdate, result Result, logger log.Logger) ([]*ServiceUpdate, error) {
	updates := []*ServiceUpdate{}

	serviceMap := a.serviceMap()
	for _, u := range candidates {
		containers, err := u.Service.ContainersOrError()
		if err != nil {
			result[u.ServiceID] = ServiceResult{
				Status: ReleaseStatusFailed,
				Error:  err.Error(),
			}
			continue
		}

		changes := serviceMap[u.ServiceID]
		containerUpdates := []ContainerUpdate{}
		for _, container := range containers {
			currentImageID, err := flux.ParseImageID(container.Image)
			if err != nil {
				return nil, err
			}

			for _, change := range changes {
				if change.container.Name != container.Name {
					continue
				}

				u.ManifestBytes, err = rc.Manifests().UpdateDefinition(u.ManifestBytes, container.Name, change.imageID)
				if err != nil {
					return nil, err
				}

				containerUpdates = append(containerUpdates, ContainerUpdate{
					Container: container.Name,
					Current:   currentImageID,
					Target:    change.imageID,
				})
			}
		}

		if len(containerUpdates) > 0 {
			u.Updates = containerUpdates
			updates = append(updates, u)
			result[u.ServiceID] = ServiceResult{
				Status:       ReleaseStatusSuccess,
				PerContainer: containerUpdates,
			}
		} else {
			result[u.ServiceID] = ServiceResult{
				Status: ReleaseStatusIgnored,
				Error:  DoesNotUseImage,
			}
		}
	}

	return updates, nil
}

func (a *Automated) serviceMap() map[flux.ServiceID][]change {
	set := map[flux.ServiceID][]change{}
	for _, change := range a.changes {
		set[change.service] = append(set[change.service], change)
	}
	return set
}

func (a *Automated) serviceIDs() []flux.ServiceID {
	slice := []flux.ServiceID{}
	for service, _ := range a.serviceMap() {
		slice = append(slice, flux.ServiceID(service.String()))
	}
	return slice
}
