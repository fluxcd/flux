package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	glob "github.com/ryanuber/go-glob"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/registry"
)

// Get the images available for the services given. An image may be
// mentioned more than once in the services, but will only be fetched
// once.
func CollectAvailableImages(reg registry.Registry, services []cluster.Service) (ImageMap, error) {
	images := ImageMap{}
	for _, service := range services {
		for _, container := range service.ContainersOrNil() {
			id, err := flux.ParseImageID(container.Image)
			if err != nil {
				// container is running an invalid image id? what?
				return nil, err
			}
			images[id.Repository()] = nil
		}
	}
	for repo := range images {
		r, err := registry.ParseRepository(repo)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing repository %s", repo)
		}
		imageRepo, err := reg.GetRepository(r)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching image metadata for %s", repo)
		}
		images[repo] = imageRepo
	}
	return images, nil
}

// TODO: Update this doc (#260)
// LatestImage returns the latest releasable image for a repository.
// A releasable image is one that is not tagged "latest". (Assumes the
// available images are in descending order of latestness.) If no such
// image exists, returns nil, and the caller can decide whether that's
// an error or not.
func (m ImageMap) LatestImage(repo, tagGlob string) *flux.Image {
	for _, image := range m[repo] {
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

// For keeping track of which images are available
type ImageMap map[string][]flux.Image

// Create a map of images. It will check that each image exists.
func ExactImages(reg registry.Registry, images []flux.ImageID) (ImageMap, error) {
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
		m[id.Repository()] = []flux.Image{flux.Image{ID: id}}
	}
	return m, nil
}

// Checks whether the given image exists in the repository.
// Return true if exist, false otherwise
func imageExists(reg registry.Registry, imageID flux.ImageID) (bool, error) {
	// Use this method to parse the image, because it is safe. I.e. it will error and inform the user if it is malformed.
	img, err := flux.ParseImage(imageID.String(), time.Time{})
	if err != nil {
		return false, err
	}
	// Get a specific image.
	_, err = reg.GetImage(registry.RepositoryFromImage(img), img.ID.Tag)
	if err != nil {
		return false, nil
	}
	return true, nil
}
