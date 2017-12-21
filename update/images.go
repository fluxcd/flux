package update

import (
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	glob "github.com/ryanuber/go-glob"

	"github.com/weaveworks/flux/cluster"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry"
)

type infoMap map[image.CanonicalName][]image.Info

type ImageMap struct {
	images infoMap
}

// LatestImage returns the latest releasable image for a repository for
// which the tag matches a given pattern. A releasable image is one that
// is not tagged "latest". (Assumes the available images are in
// descending order of latestness.) If no such image exists, returns nil,
// and the caller can decide whether that's an error or not.
func (m ImageMap) LatestImage(repo image.Name, tagGlob string) *image.Info {
	for _, available := range m.images[repo.CanonicalName()] {
		tag := available.ID.Tag
		// Ignore latest if and only if it's not what the user wants.
		if !strings.EqualFold(tagGlob, "latest") && strings.EqualFold(tag, "latest") {
			continue
		}
		if glob.Glob(tagGlob, tag) {
			var im image.Info
			im = available
			im.ID = repo.ToRef(tag)
			return &im
		}
	}
	return nil
}

// Available returns image.Info entries for all the images in the
// named image repository.
func (m ImageMap) Available(repo image.Name) []image.Info {
	if canon, ok := m.images[repo.CanonicalName()]; ok {
		infos := make([]image.Info, len(canon))
		for i := range canon {
			infos[i] = canon[i]
			infos[i].ID = repo.ToRef(infos[i].ID.Tag)
		}
		return infos
	}
	return nil
}

// collectUpdateImages is a convenient shim to
// `CollectAvailableImages`.
func collectUpdateImages(registry registry.Registry, updateable []*ControllerUpdate, logger log.Logger) (ImageMap, error) {
	var servicesToCheck []cluster.Controller
	for _, update := range updateable {
		servicesToCheck = append(servicesToCheck, update.Controller)
	}
	return CollectAvailableImages(registry, servicesToCheck, logger)
}

// CollectAvailableImages finds all the known image metadata for
// containers in the controllers given.
func CollectAvailableImages(reg registry.Registry, services []cluster.Controller, logger log.Logger) (ImageMap, error) {
	images := infoMap{}
	for _, service := range services {
		for _, container := range service.ContainersOrNil() {
			id, err := image.ParseRef(container.Image)
			if err != nil {
				// container is running an invalid image id? what?
				return ImageMap{}, err
			}
			images[id.CanonicalName()] = nil
		}
	}
	for name := range images {
		imageRepo, err := reg.GetRepository(name.Name)
		if err != nil {
			// Not an error if missing. Use empty images.
			if !fluxerr.IsMissing(err) {
				logger.Log("err", errors.Wrapf(err, "fetching image metadata for %s", name))
				continue
			}
		}
		images[name] = imageRepo
	}
	return ImageMap{images}, nil
}

// Create a map of images. It will check that each image exists.
func exactImages(reg registry.Registry, images []image.Ref) (ImageMap, error) {
	m := infoMap{}
	for _, id := range images {
		// We must check that the exact images requested actually exist. Otherwise we risk pushing invalid images to git.
		exist, err := imageExists(reg, id)
		if err != nil {
			return ImageMap{}, errors.Wrap(image.ErrInvalidImageID, err.Error())
		}
		if !exist {
			return ImageMap{}, errors.Wrap(image.ErrInvalidImageID, fmt.Sprintf("image %q does not exist", id))
		}
		m[id.CanonicalName()] = []image.Info{{ID: id}}
	}
	return ImageMap{m}, nil
}

// Checks whether the given image exists in the repository.
// Return true if exist, false otherwise
func imageExists(reg registry.Registry, imageID image.Ref) (bool, error) {
	_, err := reg.GetImage(imageID)
	if err != nil {
		return false, nil
	}
	return true, nil
}
