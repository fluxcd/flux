package update

import (
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	fluxerr "github.com/fluxcd/flux/pkg/errors"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/registry"
	"github.com/fluxcd/flux/pkg/resource"
)

type imageReposMap map[image.CanonicalName]image.RepositoryMetadata

// ImageRepos contains a map of image repositories to their metadata
type ImageRepos struct {
	imageRepos imageReposMap
}

// GetRepositoryMetadata returns the metadata for all the images in the
// named image repository.
func (r ImageRepos) GetRepositoryMetadata(repo image.Name) image.RepositoryMetadata {
	if metadata, ok := r.imageRepos[repo.CanonicalName()]; ok {
		// copy tags
		tagsCopy := make([]string, len(metadata.Tags))
		copy(tagsCopy, metadata.Tags)
		// copy images
		imagesCopy := make(map[string]image.Info, len(metadata.Images))
		for tag, info := range metadata.Images {
			// The registry (cache) stores metadata with canonical image
			// names (e.g., `index.docker.io/library/alpine`). We rewrite the
			// names based on how we were queried (repo), which could
			// be non-canonical representation (e.g. `alpine`).
			info.ID = repo.ToRef(info.ID.Tag)
			imagesCopy[tag] = info
		}
		return image.RepositoryMetadata{tagsCopy, imagesCopy}
	}
	return image.RepositoryMetadata{}
}

// SortedImageInfos is a list of sorted image.Info
type SortedImageInfos []image.Info

// FilterImages returns only the images that match the pattern, in a new list.
func FilterImages(images []image.Info, pattern policy.Pattern) []image.Info {
	return filterImages(images, pattern)
}

// SortImages orders the images according to the pattern order in a new list.
func SortImages(images []image.Info, pattern policy.Pattern) SortedImageInfos {
	return sortImages(images, pattern)
}

// FilterAndSortRepositoryMetadata obtains all the image information from the metadata
// after filtering and sorting. Filtering happens in the metadata directly to minimize
// problems with tag inconsistencies (i.e. tags without matching image information)
func FilterAndSortRepositoryMetadata(rm image.RepositoryMetadata, pattern policy.Pattern) (SortedImageInfos, error) {
	// Do the filtering
	filteredMetadata := image.RepositoryMetadata{
		Tags:   filterTags(rm.Tags, pattern),
		Images: rm.Images,
	}
	filteredImages, err := filteredMetadata.GetImageTagInfo()
	if err != nil {
		return nil, err
	}
	return SortImages(filteredImages, pattern), nil
}

// Latest returns the latest image from SortedImageInfos. If no such image exists,
// returns a zero value and `false`, and the caller can decide whether
// that's an error or not.
func (sii SortedImageInfos) Latest() (image.Info, bool) {
	if len(sii) > 0 {
		return sii[0], true
	}
	return image.Info{}, false
}

func sortImages(images []image.Info, pattern policy.Pattern) SortedImageInfos {
	var sorted SortedImageInfos
	for _, i := range images {
		sorted = append(sorted, i)
	}
	image.Sort(sorted, pattern.Newer)
	return sorted
}

func matchWithLatest(pattern policy.Pattern, tag string) bool {
	// Ignore latest if and only if it's not what the user wants.
	if pattern != policy.PatternLatest && strings.EqualFold(tag, "latest") {
		return false
	}
	return pattern.Matches(tag)
}

func filterTags(tags []string, pattern policy.Pattern) []string {
	var filtered []string
	for _, tag := range tags {
		if matchWithLatest(pattern, tag) {
			filtered = append(filtered, tag)
		}
	}
	return filtered
}

func filterImages(images []image.Info, pattern policy.Pattern) []image.Info {
	var filtered []image.Info
	for _, i := range images {
		tag := i.ID.Tag
		if matchWithLatest(pattern, tag) {
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
			imageRepos[container.Image.CanonicalName()] = image.RepositoryMetadata{}
		}
	}
	for repo := range imageRepos {
		images, err := reg.GetImageRepositoryMetadata(repo.Name)
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
		m[id.CanonicalName()] = image.RepositoryMetadata{
			Tags: []string{id.Tag},
			Images: map[string]image.Info{
				id.Tag: {ID: id},
			},
		}
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
