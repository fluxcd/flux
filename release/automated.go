package release

import (
	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
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

func (a *Automated) ServiceUpdates(rc update.ReleaseContext, logger log.Logger) ([]*update.ServiceUpdate, error) {
	filters, err := a.filters(rc)
	if err != nil {
		return nil, err
	}
	result := update.Result{}
	updates, err := rc.SelectServices(result, filters...)
	if err != nil {
		return nil, err
	}
	a.markSkipped(result)
	updates, err = a.calculateImageUpdates(rc, updates, result, logger)
	if err != nil {
		return nil, err
	}

	return updates, err
}

// TODO #260 use this
func (a *Automated) updates() map[flux.ServiceID]*update.ServiceUpdate {
	updates := map[flux.ServiceID]*update.ServiceUpdate{}
	for _, c := range a.changes {
		if _, ok := updates[c.service]; !ok {
			updates[c.service] = &update.ServiceUpdate{}
		}

		currentImageID, err := flux.ParseImageID(c.container.Image)
		if err != nil {
			// shouldn't happen
			continue
		}

		containerUpdate := update.ContainerUpdate{
			Container: c.container.Name,
			Current:   currentImageID,
			Target:    c.imageID,
		}
		serviceUpdate := updates[c.service]
		serviceUpdate.Updates = append(serviceUpdate.Updates, containerUpdate)
	}
	return updates
}

func (a *Automated) filters(rc update.ReleaseContext) ([]update.ServiceFilter, error) {
	lockedSet, err := rc.ServicesWithPolicy(policy.Locked)
	if err != nil {
		return nil, err
	}

	return []update.ServiceFilter{
		&IncludeFilter{a.serviceIDs()},
		&LockedFilter{lockedSet.ToSlice()},
	}, nil
}

func (a *Automated) markSkipped(results update.Result) {
	for _, v := range a.serviceIDs() {
		if _, ok := results[v]; !ok {
			results[v] = update.ServiceResult{
				Status: update.ReleaseStatusSkipped,
				Error:  NotInRepo,
			}
		}
	}
}

func (a *Automated) calculateImageUpdates(rc update.ReleaseContext, candidates []*update.ServiceUpdate, result update.Result, logger log.Logger) ([]*update.ServiceUpdate, error) {
	updates := []*update.ServiceUpdate{}

	serviceMap := a.serviceMap()
	for _, u := range candidates {
		containers, err := u.Service.ContainersOrError()
		if err != nil {
			result[u.ServiceID] = update.ServiceResult{
				Status: update.ReleaseStatusFailed,
				Error:  err.Error(),
			}
			continue
		}

		changes := serviceMap[u.ServiceID]
		containerUpdates := []update.ContainerUpdate{}
		for _, container := range containers {
			currentImageID, err := flux.ParseImageID(container.Image)
			if err != nil {
				return nil, err
			}

			for _, change := range changes {
				if change.container.Name != container.Name {
					continue
				}

				containerUpdates = append(containerUpdates, update.ContainerUpdate{
					Container: container.Name,
					Current:   currentImageID,
					Target:    change.imageID,
				})
			}
		}

		if len(containerUpdates) > 0 {
			u.Updates = containerUpdates
			updates = append(updates, u)
			result[u.ServiceID] = update.ServiceResult{
				Status:       update.ReleaseStatusSuccess,
				PerContainer: containerUpdates,
			}
		} else {
			result[u.ServiceID] = update.ServiceResult{
				Status: update.ReleaseStatusIgnored,
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

// TODO
func (a *Automated) Result(rc update.ReleaseContext, logger log.Logger) (update.Result, error) {
	return nil, nil
}
