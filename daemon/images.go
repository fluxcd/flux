package daemon

import (
	"context"
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
)

func (d *Daemon) pollForNewImages(logger log.Logger) {
	logger.Log("msg", "polling images")

	ctx := context.Background()

	candidateServicesPolicyMap, err := d.getUnlockedAutomatedServicesPolicyMap(ctx)
	if err != nil {
		logger.Log("error", errors.Wrap(err, "getting unlocked automated services"))
		return
	}
	if len(candidateServicesPolicyMap) == 0 {
		logger.Log("msg", "no automated services")
		return
	}
	// Find images to check
	services, err := d.Cluster.SomeControllers(candidateServicesPolicyMap.ToSlice())
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
		for _, container := range service.ContainersOrNil() {
			currentImageID := container.Image
			pattern := policy.GetTagPattern(candidateServicesPolicyMap, service.ID, container.Name)
			repo := currentImageID.Name
			logger := log.With(logger, "service", service.ID, "container", container.Name, "repo", repo, "pattern", pattern, "current", currentImageID)

			filteredImages := imageRepos.GetRepoImages(repo).Filter(pattern)

			if latest, ok := filteredImages.Latest(); ok && latest.ID != currentImageID {
				if latest.ID.Tag == "" {
					logger.Log("warning", "untagged image in available images", "action", "skip")
					continue
				}
				if latest.CreatedAt.IsZero() {
					logger.Log("warning", "image with zero created timestamp", "action", "skip")
					continue
				}
				newImage := currentImageID.WithNewTag(latest.ID.Tag)
				changes.Add(service.ID, container, newImage)
				currentCreatedAt := ""
				for _, info := range filteredImages {
					if info.ID == currentImageID {
						currentCreatedAt = info.CreatedAt.String()
					}
				}
				if currentCreatedAt == "" {
					currentCreatedAt = "filtered out or missing"
					logger.Log("warning", "current image not in filtered images", "action", "add")
				}
				logger.Log("info", "added update to automation run", "new", newImage, "reason", fmt.Sprintf("latest %s (%s) > current %s (%s)", latest.ID.Tag, latest.CreatedAt, currentImageID.Tag, currentCreatedAt))
			}
		}
	}

	if len(changes.Changes) > 0 {
		d.UpdateManifests(ctx, update.Spec{Type: update.Auto, Spec: changes})
	}
}

// getUnlockedAutomatedServicesPolicyMap returns a resource policy map for all unlocked automated services
func (d *Daemon) getUnlockedAutomatedServicesPolicyMap(ctx context.Context) (policy.ResourceMap, error) {
	var services policy.ResourceMap
	err := d.WithClone(ctx, func(checkout *git.Checkout) error {
		var err error
		services, err = d.Manifests.ServicesWithPolicies(checkout.ManifestDir())
		return err
	})
	if err != nil {
		return nil, err
	}
	automatedServices := services.OnlyWithPolicy(policy.Automated)
	lockedServices := services.OnlyWithPolicy(policy.Locked)
	return automatedServices.Without(lockedServices), nil
}
