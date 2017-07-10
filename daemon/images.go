package daemon

import (
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
)

func (d *Daemon) pollForNewImages(logger log.Logger) {
	logger.Log("msg", "polling images")

	candidateServices, err := d.unlockedAutomatedServices()
	if err != nil {
		logger.Log("error", errors.Wrap(err, "getting unlocked automated services"))
		return
	}
	if len(candidateServices) == 0 {
		logger.Log("msg", "no automated services")
		return
	}
	// Find images to check
	services, err := d.Cluster.SomeServices(candidateServices.ToSlice())
	if err != nil {
		logger.Log("error", errors.Wrap(err, "checking services for new images"))
		return
	}
	// Check the latest available image(s) for each service
	imageMap, err := update.CollectAvailableImages(d.Registry, services)
	if err != nil {
		logger.Log("error", errors.Wrap(err, "fetching image updates"))
		return
	}

	changes := &update.Automated{}
	for _, service := range services {
		for _, container := range service.ContainersOrNil() {
			logger := log.NewContext(logger).With("service", service.ID, "container", container.Name, "currentimage", container.Image)

			currentImageID, err := flux.ParseImageID(container.Image)
			if err != nil {
				logger.Log("error", err)
				continue
			}

			pattern := getTagPattern(candidateServices, service.ID, container.Name)
			repo := currentImageID.Repository()
			logger.Log("repo", repo, "pattern", pattern)

			if latest := imageMap.LatestImage(repo, pattern); latest != nil && latest.ID != currentImageID {
				changes.Add(service.ID, container, latest.ID)
				logger.Log("msg", "added image to changes", "newimage", latest.ID)
			}
		}
	}

	d.UpdateManifests(update.Spec{Type: update.Auto, Spec: changes})
}

func getTagPattern(services policy.ServiceMap, service flux.ServiceID, container string) string {
	policies := services[service]
	if pattern, ok := policies.Get(policy.TagPrefix(container)); ok {
		return strings.TrimPrefix(pattern, "glob:")
	}
	return "*"
}

func (d *Daemon) unlockedAutomatedServices() (policy.ServiceMap, error) {
	automatedServices, err := d.Manifests.ServicesWithPolicy(d.Checkout.ManifestDir(), policy.Automated)
	if err != nil {
		return nil, err
	}
	lockedServices, err := d.Manifests.ServicesWithPolicy(d.Checkout.ManifestDir(), policy.Locked)
	if err != nil {
		return nil, err
	}
	return automatedServices.Without(lockedServices), nil
}
