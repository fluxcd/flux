package daemon

import (
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/release"
	"github.com/weaveworks/flux/update"
)

func (d *Daemon) PollImages(logger log.Logger) {
	logger.Log("msg", "polling images")

	candidateServices, err := d.unlockedAutomatedServices()
	if err != nil {
		logger.Log("error", errors.Wrap(err, "getting unlocked automated services"))
		return
	}
	if len(candidateServices) == 0 {
		return
	}

	// Find images to check
	services, err := d.Cluster.SomeServices(candidateServices)
	if err != nil {
		logger.Log("error", errors.Wrap(err, "checking services for new images"))
		return
	}

	// Check the latest available image(s) for each service
	imageMap, err := release.CollectAvailableImages(d.Registry, services)
	if err != nil {
		logger.Log("error", errors.Wrap(err, "fetching image updates"))
		return
	}

	tagPatterns := d.getTagPatterns()

	// Update image for each container
	for _, service := range services {
		for _, container := range service.ContainersOrNil() {
			logger := log.NewContext(logger).With("service", service.ID, "container", container.Name, "currentimage", container.Image)

			currentImageID, err := flux.ParseImageID(container.Image)
			if err != nil {
				logger.Log("error", err)
				continue
			}

			repo := currentImageID.Repository()
			pattern := getTagPattern(tagPatterns, service.ID, container.Name)
			logger.Log("repo", repo, "pattern", pattern)

			if latest := imageMap.LatestImage(repo, pattern); latest != nil && latest.ID != currentImageID {
				if err := d.ReleaseImage(service.ID, container.Name, latest.ID); err != nil {
					logger.Log("error", err, "image", latest.ID)
				}
			}
		}
	}
}

func (d *Daemon) unlockedAutomatedServices() ([]flux.ServiceID, error) {
	automatedServices, err := d.Manifests.ServicesWithPolicy(d.Checkout.ManifestDir(), policy.Automated)
	if err != nil {
		return nil, err
	}
	lockedServices, err := d.Manifests.ServicesWithPolicy(d.Checkout.ManifestDir(), policy.Locked)
	if err != nil {
		return nil, err
	}
	return automatedServices.Without(lockedServices).ToSlice(), nil
}

func (d *Daemon) getTagPatterns() map[flux.ServiceID]map[string]string {
	serviceMap := map[flux.ServiceID]map[string]string{}
	metadata, err := d.Manifests.ServicesMetadata(d.Checkout.ManifestDir())
	if err != nil {
		return nil
	}

	for s, md := range metadata {
		serviceMap[s] = map[string]string{}

		for k, v := range md {
			if container, pattern, ok := parseTagPattern(k, v); ok {
				serviceMap[s][container] = pattern
			}
		}
	}

	return serviceMap
}

func parseTagPattern(k, v string) (string, string, bool) {
	if !strings.HasPrefix(k, "flux.weave.works/tag.") || !strings.HasPrefix(v, "glob:") {
		return "", "", false
	}

	return strings.TrimPrefix(k, "flux.weave.works/tag."), strings.TrimPrefix(v, "glob:"), true
}

func getTagPattern(patterns map[flux.ServiceID]map[string]string, serviceID flux.ServiceID, container string) string {
	if patterns == nil || patterns[serviceID] == nil {
		return "*"
	}
	if p, ok := patterns[serviceID][container]; ok {
		return p
	}
	return "*"
}

func (d *Daemon) ReleaseImage(serviceID flux.ServiceID, container string, imageID flux.ImageID) error {
	// Try to update any automated services using this image
	spec := update.ReleaseSpec{
		ServiceSpecs: []update.ServiceSpec{update.ServiceSpec(serviceID)},
		ImageSpec:    update.ImageSpecFromID(imageID),
		Kind:         update.ReleaseKindExecute,
	}
	cause := update.Cause{
		User:    update.UserAutomated,
		Message: fmt.Sprintf("Release due to new image %s", imageID.String()),
	}

	_, err := d.UpdateManifests(update.Spec{Type: update.Images, Cause: cause, Spec: spec})
	if err == git.ErrNoChanges {
		err = nil
	}
	return err
}
