package update

import (
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	glob "github.com/ryanuber/go-glob"

	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/resource"
)

type infoMap map[image.CanonicalName][]image.Info

type ImageMap struct {
	images infoMap
}

// LatestImage returns the latest releasable image for a repository
// for which the tag matches a given pattern. A releasable image is
// one that is not tagged "latest". (Assumes the available images are
// in descending order of latestness.) If no such image exists,
// returns a zero value and `false`, and the caller can decide whether
// that's an error or not.
func (m ImageMap) LatestImage(repo image.Name, tagGlob string) (image.Info, bool) {
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
			return im, true
		}
	}
	return image.Info{}, false
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

// containers represents a collection of things that have containers
type containers interface {
	Len() int
	Containers(i int) []resource.Container
}

type controllerContainers []*ControllerUpdate

func (cs controllerContainers) Len() int {
	return len(cs)
}

func (cs controllerContainers) Containers(i int) []resource.Container {
	return cs[i].Controller.ContainersOrNil()
}

// collectUpdateImages is a convenient shim to
// `CollectAvailableImages`.
func collectUpdateImages(registry registry.Registry, updateable []*ControllerUpdate, logger log.Logger) (ImageMap, error) {
	return CollectAvailableImages(registry, controllerContainers(updateable), logger)
}

// CollectAvailableImages finds all the known image metadata for
// containers in the controllers given.
func CollectAvailableImages(reg registry.Registry, cs containers, logger log.Logger) (ImageMap, error) {
	images := infoMap{}
	for i := 0; i < cs.Len(); i++ {
		for _, container := range cs.Containers(i) {
			images[container.Image.CanonicalName()] = nil
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
// Return true if exist, false otherwise.
// FIXME(michael): never returns an error; should it?
func imageExists(reg registry.Registry, imageID image.Ref) (bool, error) {
	_, err := reg.GetImage(imageID)
	if err != nil {
		return false, nil
	}
	return true, nil
}
