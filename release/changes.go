package release

import (
	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
)

type Changes struct {
	changes []change
}

type change struct {
	service   flux.ServiceID
	container cluster.Container
	imageID   flux.ImageID
}

func (c *Changes) Add(service flux.ServiceID, container cluster.Container, image flux.ImageID) {
	c.changes = append(c.changes, change{service, container, image})
}

func (c *Changes) ServiceUpdates(rc *ReleaseContext, logger log.Logger) ([]*ServiceUpdate, error) {
	filters, err := c.filters(rc)
	if err != nil {
		return nil, err
	}
	result := update.Result{}
	updates, err := rc.SelectServices(result, filters...)
	if err != nil {
		return nil, err
	}
	c.markSkipped(result)
	updates, err = c.calculateImageUpdates(rc, updates, result, logger)
	if err != nil {
		return nil, err
	}

	return updates, err
}

// TODO #260 use this
func (c *Changes) updates() map[flux.ServiceID]*ServiceUpdate {
	updates := map[flux.ServiceID]*ServiceUpdate{}
	for _, c := range c.changes {
		if _, ok := updates[c.service]; !ok {
			updates[c.service] = &ServiceUpdate{}
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

func (c *Changes) filters(rc *ReleaseContext) ([]ServiceFilter, error) {
	lockedSet, err := rc.ServicesWithPolicy(policy.Locked)
	if err != nil {
		return nil, err
	}

	return []ServiceFilter{
		&IncludeFilter{c.serviceIDs()},
		&LockedFilter{lockedSet.ToSlice()},
	}, nil
}

func (c *Changes) markSkipped(results update.Result) {
	for _, v := range c.serviceIDs() {
		if _, ok := results[v]; !ok {
			results[v] = update.ServiceResult{
				Status: update.ReleaseStatusSkipped,
				Error:  NotInRepo,
			}
		}
	}
}

func (c *Changes) calculateImageUpdates(rc *ReleaseContext, candidates []*ServiceUpdate, result update.Result, logger log.Logger) ([]*ServiceUpdate, error) {
	updates := []*ServiceUpdate{}

	serviceMap := c.serviceMap()
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

func (c *Changes) serviceMap() map[flux.ServiceID][]change {
	set := map[flux.ServiceID][]change{}
	for _, change := range c.changes {
		set[change.service] = append(set[change.service], change)
	}
	return set
}

func (c *Changes) serviceIDs() []flux.ServiceID {
	slice := []flux.ServiceID{}
	for service, _ := range c.serviceMap() {
		slice = append(slice, flux.ServiceID(service.String()))
	}
	return slice

}
