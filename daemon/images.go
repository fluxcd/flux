package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
)

func (d *Daemon) pollForNewImages(logger log.Logger) {
	fmt.Println("\n--------------- in pollForNewImages")

	logger.Log("msg", "polling images")

	ctx := context.Background()

	candidateServices, err := d.unlockedAutomatedServices(ctx)
	fmt.Printf("\t\t automatedServicesWithout (candidateServices) ... %+v\n", candidateServices)

	if err != nil {
		logger.Log("error", errors.Wrap(err, "getting unlocked automated services"))
		return
	}
	if len(candidateServices) == 0 {
		logger.Log("msg", "no automated services")
		return
	}
	// Find images to check
	services, err := d.Cluster.SomeControllers(candidateServices.ToSlice())
	fmt.Printf("\n==========\t\tservices from SomeControllers ... %+v\n==========\n", services)

	if err != nil {
		logger.Log("error", errors.Wrap(err, "checking services for new images"))
		return
	}
	// Check the latest available image(s) for each service
	imageMap, err := update.CollectAvailableImages(d.Registry, clusterContainers(services), logger)
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

			pattern := getTagPattern(candidateServices, service.ID, container.Name)
			repo := currentImageID.Name
			logger.Log("repo", repo, "pattern", pattern)

			if latest, ok := imageMap.LatestImage(repo, pattern); ok && latest.ID != currentImageID {
				fmt.Printf("\n\t\t\t latest image = %+v\n========\n", latest)
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

	fmt.Printf("\n========\t\t\tdaemon/images.go, pollForNewImages: changes = %+v\n========\n", changes.Changes)

	if len(changes.Changes) > 0 {
		fmt.Println("\t\t >>> There should be AUTOMATED changes!!!")
		d.UpdateManifests(ctx, update.Spec{Type: update.Auto, Spec: changes})
	}
}

func getTagPattern(services policy.ResourceMap, service flux.ResourceID, container string) string {
	policies := services[service]
	if pattern, ok := policies.Get(policy.TagPrefix(container)); ok {
		return strings.TrimPrefix(pattern, "glob:")
	}
	return "*"
}

func (d *Daemon) unlockedAutomatedServices(ctx context.Context) (policy.ResourceMap, error) {
	fmt.Println("\n-------------- unlockedAutomatedServices -----------------\n")
	var services policy.ResourceMap
	err := d.WithClone(ctx, func(checkout *git.Checkout) error {
		var err error
		services, err = d.Manifests.ServicesWithPolicies(checkout.ManifestDir())
		fmt.Printf("\t\tservices ... %+v\n", services)
		fmt.Printf("\t\terr ... %+v\n", err)
		return err
	})
	if err != nil {
		return nil, err
	}
	automatedServices := services.OnlyWithPolicy(policy.Automated)
	fmt.Printf("\t\tautomated services ... %+v\n", automatedServices)
	lockedServices := services.OnlyWithPolicy(policy.Locked)
	fmt.Printf("\t\tlocked services ... %+v\n", lockedServices)

	fmt.Println("\n-------------- END unlockedAutomatedServices -----------------\n")

	return automatedServices.Without(lockedServices), nil
}
