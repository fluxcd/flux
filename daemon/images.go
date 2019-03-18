package daemon

import (
	"context"
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/update"
)

func (d *Daemon) pollForNewImages(logger log.Logger) {
	logger.Log("msg", "polling images")

	ctx := context.Background()

	candidateWorkloads, err := d.getAllowedAutomatedResources(ctx)
	if err != nil {
		logger.Log("error", errors.Wrap(err, "getting unlocked automated resources"))
		return
	}
	if len(candidateWorkloads) == 0 {
		logger.Log("msg", "no automated workloads")
		return
	}
	// Find images to check
	workloads, err := d.Cluster.SomeWorkloads(candidateWorkloads.IDs())
	if err != nil {
		logger.Log("error", errors.Wrap(err, "checking workloads for new images"))
		return
	}
	// Check the latest available image(s) for each workload
	imageRepos, err := update.FetchImageRepos(d.Registry, clusterContainers(workloads), logger)
	if err != nil {
		logger.Log("error", errors.Wrap(err, "fetching image updates"))
		return
	}

	changes := calculateChanges(logger, candidateWorkloads, workloads, imageRepos)

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

// getAllowedAutomatedResources returns all the resources that are
// automated but do not have policies set to restrain them from
// getting updated.
func (d *Daemon) getAllowedAutomatedResources(ctx context.Context) (resources, error) {
	resources, _, err := d.getResources(ctx)
	if err != nil {
		return nil, err
	}

	result := map[flux.ResourceID]resource.Resource{}
	for _, resource := range resources {
		policies := resource.Policies()
		if policies.Has(policy.Automated) && !policies.Has(policy.Locked) && !policies.Has(policy.Ignore) {
			result[resource.ResourceID()] = resource
		}
	}
	return result, nil
}

func calculateChanges(logger log.Logger, candidateWorkloads resources, workloads []cluster.Workload, imageRepos update.ImageRepos) *update.Automated {
	changes := &update.Automated{}

	for _, workload := range workloads {
		var p policy.Set
		if resource, ok := candidateWorkloads[workload.ID]; ok {
			p = resource.Policies()
		}
	containers:
		for _, container := range workload.ContainersOrNil() {
			currentImageID := container.Image
			pattern := policy.GetTagPattern(p, container.Name)
			repo := currentImageID.Name
			logger := log.With(logger, "workload", workload.ID, "container", container.Name, "repo", repo, "pattern", pattern, "current", currentImageID)

			images := imageRepos.GetRepoImages(repo)
			filteredImages := images.FilterAndSort(pattern)

			if latest, ok := filteredImages.Latest(); ok && latest.ID != currentImageID {
				if latest.ID.Tag == "" {
					logger.Log("warning", "untagged image in available images", "action", "skip container")
					continue containers
				}
				current := images.FindWithRef(currentImageID)
				if current.CreatedAt.IsZero() || latest.CreatedAt.IsZero() {
					logger.Log("warning", "image with zero created timestamp", "current", fmt.Sprintf("%s (%s)", current.ID, current.CreatedAt), "latest", fmt.Sprintf("%s (%s)", latest.ID, latest.CreatedAt), "action", "skip container")
					continue containers
				}
				newImage := currentImageID.WithNewTag(latest.ID.Tag)
				changes.Add(workload.ID, container, newImage)
				logger.Log("info", "added update to automation run", "new", newImage, "reason", fmt.Sprintf("latest %s (%s) > current %s (%s)", latest.ID.Tag, latest.CreatedAt, currentImageID.Tag, current.CreatedAt))
			}
		}
	}

	return changes
}
