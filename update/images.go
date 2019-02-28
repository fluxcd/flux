package update

import (
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
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

// SortedImageInfos is a list of sorted image.Info
type SortedImageInfos []image.Info

// Filter returns only the images that match the pattern, in a new list.
func (ii ImageInfos) Filter(pattern policy.Pattern) ImageInfos {
	return filterImages(ii, pattern)
}

// Sort orders the images according to the pattern order in a new list.
func (ii ImageInfos) Sort(pattern policy.Pattern) SortedImageInfos {
	return sortImages(ii, pattern)
}

// FilterAndSort is an optimized helper function to compose filtering and sorting.
func (ii ImageInfos) FilterAndSort(pattern policy.Pattern) SortedImageInfos {
	filtered := ii.Filter(pattern)
	// Do not call sortImages() here which will clone the list that we already
	// cloned in ImageInfos.Filter()
	image.Sort(filtered, pattern.Newer)
	return SortedImageInfos(filtered)
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

// Latest returns the latest image from SortedImageInfos. If no such image exists,
// returns a zero value and `false`, and the caller can decide whether
// that's an error or not.
func (is SortedImageInfos) Latest() (image.Info, bool) {
	if len(is) > 0 {
		return is[0], true
	}
	return image.Info{}, false
}

// Filter returns only the images that match the pattern, in a new list.
func (is SortedImageInfos) Filter(pattern policy.Pattern) SortedImageInfos {
	return SortedImageInfos(filterImages(is, pattern))
}

// Sort orders the images according to the pattern order in a new list.
func (is SortedImageInfos) Sort(pattern policy.Pattern) SortedImageInfos {
	return sortImages(is, pattern)
}

func sortImages(images []image.Info, pattern policy.Pattern) SortedImageInfos {
	var sorted SortedImageInfos
	for _, i := range images {
		sorted = append(sorted, i)
	}
	image.Sort(sorted, pattern.Newer)
	return sorted
}

// filterImages keeps the sort order pristine.
func filterImages(images []image.Info, pattern policy.Pattern) ImageInfos {
	var filtered ImageInfos
	for _, i := range images {
		tag := i.ID.Tag
		// Ignore latest if and only if it's not what the user wants.
		if pattern != policy.PatternLatest && strings.EqualFold(tag, "latest") {
			continue
		}
		if pattern.Matches(tag) {
			filtered = append(filtered, i)
		}
	}
	return filtered
}

// containers represents a collection of things that have containers
type containers interface {
	Len() int
	Containers(i int) []resource.Container
}

type workloadContainers []*WorkloadUpdate

func (cs workloadContainers) Len() int {
	return len(cs)
}

func (cs workloadContainers) Containers(i int) []resource.Container {
	return cs[i].Workload.ContainersOrNil()
}

// fetchUpdatableImageRepos is a convenient shim to
// `FetchImageRepos`.
func fetchUpdatableImageRepos(registry registry.Registry, updateable []*WorkloadUpdate, logger log.Logger) (ImageRepos, error) {
	return FetchImageRepos(registry, workloadContainers(updateable), logger)
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
		images, err := reg.GetRepositoryImages(repo.Name)
		if err != nil {
			// Not an error if missing. Use empty images.
			if !fluxerr.IsMissing(err) {
				logger.Log("err", errors.Wrapf(err, "fetching image metadata for %s", repo))
				continue
			}
		}
		imageRepos[repo] = images
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
