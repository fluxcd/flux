package update

import (
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	glob "github.com/ryanuber/go-glob"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/registry"
)

type ImageMap map[flux.CanonicalName][]flux.Image

// LatestImage returns the latest releasable image for a repository for
// which the tag matches a given pattern. A releasable image is one that
// is not tagged "latest". (Assumes the available images are in
// descending order of latestness.) If no such image exists, returns nil,
// and the caller can decide whether that's an error or not.
func (m ImageMap) LatestImage(repo flux.ImageName, tagGlob string) *flux.Image {
	for _, image := range m[repo.CanonicalName()] {
		_, _, tag := image.ID.Components()
		// Ignore latest if and only if it's not what the user wants.
		if !strings.EqualFold(tagGlob, "latest") && strings.EqualFold(tag, "latest") {
			continue
		}
		if glob.Glob(tagGlob, tag) {
			return &image
		}
	}
	return nil
}

// CollectUpdateImages is a convenient shim to
// `CollectAvailableImages`.
func collectUpdateImages(registry registry.Registry, updateable []*ControllerUpdate, logger log.Logger) (ImageMap, error) {
	var servicesToCheck []cluster.Controller
	for _, update := range updateable {
		servicesToCheck = append(servicesToCheck, update.Controller)
	}
	return CollectAvailableImages(registry, servicesToCheck, logger)
}

// Get the images available for the services given. An image may be
// mentioned more than once in the services, but will only be fetched
// once.
func CollectAvailableImages(reg registry.Registry, services []cluster.Controller, logger log.Logger) (ImageMap, error) {
	images := ImageMap{}
	for _, service := range services {
		for _, container := range service.ContainersOrNil() {
			id, err := flux.ParseImageRef(container.Image)
			if err != nil {
				// container is running an invalid image id? what?
				return nil, err
			}
			images[id.CanonicalName()] = nil
		}
	}
	for name := range images {
		imageRepo, err := reg.GetRepository(name.ImageName)
		if err != nil {
			// Not an error if missing. Use empty images.
			if !fluxerr.IsMissing(err) {
				logger.Log("err", errors.Wrapf(err, "fetching image metadata for %s", name))
				continue
			}
		}
		images[name] = imageRepo
	}
	return images, nil
}

// Create a map of images. It will check that each image exists.
func exactImages(reg registry.Registry, images []flux.ImageRef) (ImageMap, error) {
	m := ImageMap{}
	for _, id := range images {
		// We must check that the exact images requested actually exist. Otherwise we risk pushing invalid images to git.
		exist, err := imageExists(reg, id)
		if err != nil {
			return m, errors.Wrap(flux.ErrInvalidImageID, err.Error())
		}
		if !exist {
			return m, errors.Wrap(flux.ErrInvalidImageID, fmt.Sprintf("image %q does not exist", id))
		}
		m[id.CanonicalName()] = []flux.Image{{ID: id}}
	}
	return m, nil
}

// Checks whether the given image exists in the repository.
// Return true if exist, false otherwise
func imageExists(reg registry.Registry, imageID flux.ImageRef) (bool, error) {
	_, err := reg.GetImage(imageID)
	if err != nil {
		return false, nil
	}
	return true, nil
}
