package daemon

import (
	"context"
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
)

func (d *Daemon) pollForNewAutomatedWorkloadImages(logger log.Logger) {
	logger.Log("msg", "polling for new images for automated workloads")

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
	workloads, err := d.Cluster.SomeWorkloads(ctx, candidateWorkloads.IDs())
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

type resources map[resource.ID]resource.Resource

func (r resources) IDs() (ids []resource.ID) {
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

	result := map[resource.ID]resource.Resource{}
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
			repoMetadata := imageRepos.GetRepositoryMetadata(repo)
			images, err := update.FilterAndSortRepositoryMetadata(repoMetadata, pattern)
			if err != nil {
				logger.Log("warning", fmt.Sprintf("inconsistent repository metadata: %s", err), "action", "skip container")
				continue containers
			}

			if latest, ok := images.Latest(); ok && latest.ID != currentImageID {
				if latest.ID.Tag == "" {
					logger.Log("warning", "untagged image in available images", "action", "skip container")
					continue containers
				}
				current := repoMetadata.FindImageWithRef(currentImageID)
				if pattern.RequiresTimestamp() && (current.CreatedAt.IsZero() || latest.CreatedAt.IsZero()) {
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
