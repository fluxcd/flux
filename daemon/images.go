package daemon

import (
	"context"
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/update"
)

func (d *Daemon) pollForNewImages(logger log.Logger) {
	logger.Log("msg", "polling images")

	ctx := context.Background()

	candidateServices, err := d.getUnlockedAutomatedResources(ctx)
	if err != nil {
		logger.Log("error", errors.Wrap(err, "getting unlocked automated resources"))
		return
	}
	if len(candidateServices) == 0 {
		logger.Log("msg", "no automated services")
		return
	}
	// Find images to check
	services, err := d.Cluster.SomeControllers(candidateServices.IDs())
	if err != nil {
		logger.Log("error", errors.Wrap(err, "checking services for new images"))
		return
	}
	// Check the latest available image(s) for each service
	imageRepos, err := update.FetchImageRepos(d.Registry, clusterContainers(services), logger)
	if err != nil {
		logger.Log("error", errors.Wrap(err, "fetching image updates"))
		return
	}

	changes := &update.Automated{}
	for _, service := range services {
		var p policy.Set
		if resource, ok := candidateServices[service.ID]; ok {
			p = resource.Policy()
		}
	containers:
		for _, container := range service.ContainersOrNil() {
			currentImageID := container.Image
			pattern := policy.GetTagPattern(p, container.Name)
			repo := currentImageID.Name
			logger := log.With(logger, "service", service.ID, "container", container.Name, "repo", repo, "pattern", pattern, "current", currentImageID)

			filteredImages := imageRepos.GetRepoImages(repo).FilterAndSort(pattern)

			if latest, ok := filteredImages.Latest(); ok && latest.ID != currentImageID {
				if latest.ID.Tag == "" {
					logger.Log("warning", "untagged image in available images", "action", "skip container")
					continue containers
				}
				currentCreatedAt := ""
				for _, info := range filteredImages {
					if info.CreatedAt.IsZero() {
						logger.Log("warning", "image with zero created timestamp", "image", info.ID, "action", "skip container")
						continue containers
					}
					if info.ID == currentImageID {
						currentCreatedAt = info.CreatedAt.String()
					}
				}
				if currentCreatedAt == "" {
					currentCreatedAt = "filtered out or missing"
					logger.Log("warning", "current image not in filtered images", "action", "proceed anyway")
				}
				newImage := currentImageID.WithNewTag(latest.ID.Tag)
				changes.Add(service.ID, container, newImage)
				logger.Log("info", "added update to automation run", "new", newImage, "reason", fmt.Sprintf("latest %s (%s) > current %s (%s)", latest.ID.Tag, latest.CreatedAt, currentImageID.Tag, currentCreatedAt))
			}
		}
	}

	if len(changes.Changes) > 0 {
		d.UpdateManifests(ctx, update.Spec{Type: update.Auto, Spec: changes})
	}
}

type resources map[flux.ResourceID]resource.Resource

func (r resources) IDs() (ids []flux.ResourceID) {
	for k, _ := range r {
		ids = append(ids, k)
	}
	return ids
}

// getUnlockedAutomatedServices returns all the resources that are
// both automated, and not locked.
func (d *Daemon) getUnlockedAutomatedResources(ctx context.Context) (resources, error) {
	resources, _, err := d.getResources(ctx)
	if err != nil {
		return nil, err
	}

	result := map[flux.ResourceID]resource.Resource{}
	for _, resource := range resources {
		policies := resource.Policy()
		if policies.Has(policy.Automated) && !policies.Has(policy.Locked) {
			result[resource.ResourceID()] = resource
		}
	}
	return result, nil
}
