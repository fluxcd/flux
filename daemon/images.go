package daemon

import (
	"context"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
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
			logger := log.With(logger, "service", service.ID, "container", container.Name, "currentimage", container.Image)

			currentImageID := container.Image
			if err != nil {
				logger.Log("error", err)
				continue
			}

			pattern := getTagPattern(candidateServicesPolicyMap, service.ID, container.Name)
			repo := currentImageID.Name
			logger.Log("repo", repo, "pattern", pattern)

			filteredImages := imageRepos.GetRepoImages(repo).Filter(pattern)

			if latest, ok := filteredImages.Latest(); ok && latest.ID != currentImageID {
				if latest.ID.Tag == "" {
					logger.Log("msg", "untagged image in available images", "action", "skip", "available", repo)
					continue
				}
				newImage := currentImageID.WithNewTag(latest.ID.Tag)
				changes.Add(service.ID, container, newImage)
				logger.Log("msg", "added image to changes", "newimage", newImage)
			}
		}
	}

	if len(changes.Changes) > 0 {
		d.UpdateManifests(ctx, update.Spec{Type: update.Auto, Spec: changes})
	}
}

func getTagPattern(services policy.ResourceMap, service flux.ResourceID, container string) string {
	if services == nil {
		return "*"
	}
	policies := services[service]
	if pattern, ok := policies.Get(policy.TagPrefix(container)); ok {
		return strings.TrimPrefix(pattern, "glob:")
	}
	return "*"
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
