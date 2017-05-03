package daemon

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/release"
	"github.com/weaveworks/flux/update"
)

func (d *Daemon) PollImages() {
	automatedServices, err := d.Cluster.ServicesWithPolicy(d.Checkout.ManifestDir(), policy.Automated)
	if err != nil {
		log.Error(errors.Wrap(err, "checking service policies"))
		return
	}
	lockedServices, err := d.Cluster.ServicesWithPolicy(d.Checkout.ManifestDir(), policy.Locked)
	if err != nil {
		log.Error(errors.Wrap(err, "checking service policies"))
		return
	}

	// Any services automated?
	candidateServices := automatedServices.Without(lockedServices).ToSlice()
	if len(candidateServices) == 0 {
		return
	}

	// Find images to check
	services, err := d.Cluster.SomeServices(candidateServices)
	if err != nil {
		log.Error(errors.Wrap(err, "checking services for new images"))
		return
	}

	// Check the latest available image(s) for each service
	images, err := release.CollectAvailableImages(d.Registry, services)
	if err != nil {
		log.Error(errors.Wrap(err, "fetching image updates"))
		return
	}

	// Are any of the images new?
	newImages := map[flux.ImageID]struct{}{}
	for _, service := range services {
		for _, container := range service.ContainersOrNil() {
			currentImageID, err := flux.ParseImageID(container.Image)
			if err != nil {
				log.Error(errors.Wrapf(err, "parsing image in service %s container %s (%q)", service.ID, container.Name, container.Image))
				continue
			}
			if latest := images.LatestImage(currentImageID.Repository()); latest != nil && latest.ID != currentImageID {
				newImages[latest.ID] = struct{}{}
			}
		}
	}

	// Release the new image(s)
	for imageID := range newImages {
		if err := d.NewImage(imageID); err != nil {
			log.Error(errors.Wrapf(err, "releasing new image %s", imageID))
		}
	}
}

func (d *Daemon) NewImage(imageID flux.ImageID) error {
	// Try to update any automated services using this image
	spec := flux.ReleaseSpec{
		ServiceSpecs: []flux.ServiceSpec{flux.ServiceSpecAutomated},
		ImageSpec:    flux.ImageSpecFromID(imageID),
		Kind:         flux.ReleaseKindExecute,
		Cause: flux.ReleaseCause{
			User:    flux.UserAutomated,
			Message: fmt.Sprintf("due to new image %s", imageID.String()),
		},
	}
	_, err := d.UpdateManifests(update.Spec{Type: update.Images, Spec: spec})
	if err == git.ErrNoChanges {
		err = nil
	}
	return err
}
