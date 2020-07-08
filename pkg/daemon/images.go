package daemon

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
)

func (d *Daemon) pollForNewAutomatedWorkloadImages(logger *zap.Logger) {
	logger.Info("polling for new images for automated workloads")

	ctx := context.Background()

	candidateWorkloads, err := d.getAllowedAutomatedResources(ctx)
	if err != nil {
		logger.Error(
			"error getting unlocked automated resources",
			zap.NamedError("err", err),
		)
		return
	}
	if len(candidateWorkloads) == 0 {
		logger.Info("no automated workloads")
		return
	}
	// Find images to check
	workloads, err := d.Cluster.SomeWorkloads(ctx, candidateWorkloads.IDs())
	if err != nil {
		logger.Error(
			"error checking workloads for new images",
			zap.NamedError("err", err),
		)
		return
	}
	// Check the latest available image(s) for each workload
	imageRepos, err := update.FetchImageRepos(d.Registry, clusterContainers(workloads), logger)
	if err != nil {
		logger.Error(
			"error fetching image updates",
			zap.NamedError("err", err),
		)
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

func calculateChanges(logger *zap.Logger, candidateWorkloads resources, workloads []cluster.Workload, imageRepos update.ImageRepos) *update.Automated {
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
			logger := logger.With(
				zap.Any("workload", workload.ID),
				zap.String("container", container.Name),
				zap.Any("repo", repo),
				zap.Any("pattern", pattern),
				zap.Any("current", currentImageID),
			)
			repoMetadata := imageRepos.GetRepositoryMetadata(repo)
			images, err := update.FilterAndSortRepositoryMetadata(repoMetadata, pattern)
			if err != nil {
				logger.Warn(fmt.Sprintf("inconsistent repository metadata: %s", err), zap.String("action", "skip container"))
				continue containers
			}

			if latest, ok := images.Latest(); ok && latest.ID != currentImageID {
				if latest.ID.Tag == "" {
					logger.Warn("untagged image in available images", zap.String("action", "skip container"))
					continue containers
				}
				current := repoMetadata.FindImageWithRef(currentImageID)
				if pattern.RequiresTimestamp() && (current.CreatedAt.IsZero() || latest.CreatedAt.IsZero()) {
					logger.Warn(
						"image with zero created timestamp",
						zap.String("current", fmt.Sprintf("%s (%s)", current.ID, current.CreatedAt)),
						zap.String("latest", fmt.Sprintf("%s (%s)", latest.ID, latest.CreatedAt)),
						zap.String("action", "skip container"),
					)
					continue containers
				}
				newImage := currentImageID.WithNewTag(latest.ID.Tag)
				changes.Add(workload.ID, container, newImage)
				logger.Info(
					"added update to automation run",
					zap.Any("new", newImage),
					zap.String("reason", fmt.Sprintf("latest %s (%s) > current %s (%s)", latest.ID.Tag, latest.CreatedAt, currentImageID.Tag, current.CreatedAt)),
				)
			}
		}
	}

	return changes
}
