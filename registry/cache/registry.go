package cache

import (
	"encoding/json"
	"sort"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux/image"
)

var (
	ErrNotCached = &fluxerr.Error{
		Type: fluxerr.Missing,
		Err:  errors.New("item not in cache"),
		Help: `Image not yet cached

It takes time to initially cache all the images. Please wait.

If you have waited for a long time, check the flux logs. Potential
reasons for the error are: no internet, no cache, error with the remote
repository.
`,
	}
)

// Cache is a local cache of image metadata.
type Cache struct {
	Reader Reader
}

// GetRepository returns the list of image manifests in an image
// repository (e.g,. at "quay.io/weaveworks/flux")
func (c *Cache) GetRepository(id image.Name) ([]image.Info, error) {
	repoKey := NewRepositoryKey(id.CanonicalName())
	bytes, _, err := c.Reader.GetKey(repoKey)
	if err != nil {
		return nil, err
	}
	var repo ImageRepository
	if err = json.Unmarshal(bytes, &repo); err != nil {
		return nil, err
	}

	// We only care about the error if we've never successfully
	// updated the result.
	if repo.LastUpdate.IsZero() {
		if repo.LastError != "" {
			return nil, errors.New(repo.LastError)
		}
		return nil, ErNotCached
	}

	images := make([]image.Info, len(repo.Images))
	var i int
	for _, im := range repo.Images {
		images[i] = im
		i++
	}
	sort.Sort(image.ByCreatedDesc(images))
	return images, nil
}

// GetImage gets the manifest of a specific image ref, from its
// registry.
func (c *Cache) GetImage(id image.Ref) (image.Info, error) {
	key := NewManifestKey(id.CanonicalRef())

	val, _, err := c.Reader.GetKey(key)
	if err != nil {
		return image.Info{}, err
	}
	var img image.Info
	err = json.Unmarshal(val, &img)
	if err != nil {
		return image.Info{}, err
	}
	return img, nil
}

// ImageRepository holds the last good information on an image
// repository.
//
// Whenever we successfully fetch a full set of image info,
// `LastUpdate` and `Images` shall each be assigned a value, and
// `LastError` will be cleared.
//
// If we cannot for any reason obtain a full set of image info,
// `LastError` shall be assigned a value, and the other fields left
// alone.
//
// It's possible to have all fields populated: this means at some
// point it was successfully fetched, but since then, there's been an
// error. It's then up to the caller to decide what to do with the
// value (show the images, but also indicate there's a problem, for
// example).
type ImageRepository struct {
	LastError  string
	LastUpdate time.Time
	Images     map[string]image.Info
}
