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

type imageReposMap map[image.CanonicalName]ImageInfos

// ImageRepos contains a map of image repositories to their images
type ImageRepos struct {
	imageRepos imageReposMap
}

// GetRepoImages returns image.Info entries for all the images in the
// named image repository.
func (r ImageRepos) GetRepoImages(repo image.Name) ImageInfos {
	if canon, ok := r.imageRepos[repo.CanonicalName()]; ok {
		infos := make([]image.Info, len(canon))
		for i := range canon {
			infos[i] = canon[i]
			infos[i].ID = repo.ToRef(infos[i].ID.Tag)
		}
		return infos
	}
	return nil
}

// ImageInfos is a list of image.Info which can be filtered.
type ImageInfos []image.Info

// Filter returns only the images which match the tagGlob.
func (ii ImageInfos) Filter(tagGlob string) ImageInfos {
	var filtered ImageInfos
	for _, i := range ii {
		tag := i.ID.Tag
		// Ignore latest if and only if it's not what the user wants.
		if !strings.EqualFold(tagGlob, "latest") && strings.EqualFold(tag, "latest") {
			continue
		}
		if glob.Glob(tagGlob, tag) {
			var im image.Info
			im = i
			filtered = append(filtered, im)
		}
	}
	return filtered
}

// Latest returns the latest image from ImageInfos. If no such image exists,
// returns a zero value and `false`, and the caller can decide whether
// that's an error or not.
func (ii ImageInfos) Latest() (image.Info, bool) {
	if len(ii) > 0 {
		return ii[0], true
	}
	return image.Info{}, false
}

// FindWithRef returns image.Info given an image ref. If the image cannot be
// found, it returns the image.Info with the ID provided.
func (ii ImageInfos) FindWithRef(ref image.Ref) image.Info {
	for _, img := range ii {
		if img.ID == ref {
			return img
		}
	}
	return image.Info{ID: ref}
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

// fetchUpdatableImageRepos is a convenient shim to
// `FetchImageRepos`.
func fetchUpdatableImageRepos(registry registry.Registry, updateable []*ControllerUpdate, logger log.Logger) (ImageRepos, error) {
	return FetchImageRepos(registry, controllerContainers(updateable), logger)
}

// FetchImageRepos finds all the known image metadata for
// containers in the controllers given.
func FetchImageRepos(reg registry.Registry, cs containers, logger log.Logger) (ImageRepos, error) {
	imageRepos := imageReposMap{}
	for i := 0; i < cs.Len(); i++ {
		for _, container := range cs.Containers(i) {
			imageRepos[container.Image.CanonicalName()] = nil
		}
	}
	for repo := range imageRepos {
		sortedRepoImages, err := reg.GetSortedRepositoryImages(repo.Name)
		if err != nil {
			// Not an error if missing. Use empty images.
			if !fluxerr.IsMissing(err) {
				logger.Log("err", errors.Wrapf(err, "fetching image metadata for %s", repo))
				continue
			}
		}
		imageRepos[repo] = sortedRepoImages
	}
	return ImageRepos{imageRepos}, nil
}

// Create a map of image repos to images. It will check that each image exists.
func exactImageRepos(reg registry.Registry, images []image.Ref) (ImageRepos, error) {
	m := imageReposMap{}
	for _, id := range images {
		// We must check that the exact images requested actually exist. Otherwise we risk pushing invalid images to git.
		exist, err := imageExists(reg, id)
		if err != nil {
			return ImageRepos{}, errors.Wrap(image.ErrInvalidImageID, err.Error())
		}
		if !exist {
			return ImageRepos{}, errors.Wrap(image.ErrInvalidImageID, fmt.Sprintf("image %q does not exist", id))
		}
		m[id.CanonicalName()] = []image.Info{{ID: id}}
	}
	return ImageRepos{m}, nil
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
