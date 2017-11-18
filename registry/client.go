package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry/cache"
)

// ---

// Client is a registry client. It is an interface so we can wrap it
// in instrumentation, write fake implementations, and so on.
type Client interface {
	Tags(name image.Name) ([]string, error)
	Manifest(name image.Ref) (image.Info, error)
	Cancel()
}

// An implementation of Client that represents a Remote registry.
// E.g. docker hub.
type Remote struct {
	Registry   *herokuManifestAdaptor
	CancelFunc context.CancelFunc
}

// Return the tags for this repository.
func (a *Remote) Tags(id image.Name) ([]string, error) {
	return a.Registry.Tags(id.Repository())
}

// We need to do some adapting here to convert from the return values
// from dockerregistry to our domain types.
func (a *Remote) Manifest(id image.Ref) (image.Info, error) {
	manifestV2, err := a.Registry.ManifestV2(id.Repository(), id.Tag)
	if err != nil {
		if err, ok := err.(*url.Error); ok {
			if err, ok := (err.Err).(*dockerregistry.HttpStatusError); ok {
				if err.Response.StatusCode == http.StatusNotFound {
					return a.ManifestFromV1(id)
				}
			}
		}
		return image.Info{}, err
	}
	// The above request will happily return a bogus, empty manifest
	// if handed something other than a schema2 manifest.
	if manifestV2.Config.Digest == "" {
		return a.ManifestFromV1(id)
	}

	// schema2 manifests have a reference to a blog that contains the
	// image config. We have to fetch that in order to get the created
	// datetime.
	conf := manifestV2.Config
	reader, err := a.Registry.DownloadLayer(id.Repository(), conf.Digest)
	if err != nil {
		return image.Info{}, err
	}
	if reader == nil {
		return image.Info{}, fmt.Errorf("nil reader from DownloadLayer")
	}

	type config struct {
		Created time.Time `json:created`
	}
	var imageConf config

	err = json.NewDecoder(reader).Decode(&imageConf)
	if err != nil {
		return image.Info{}, err
	}
	return image.Info{
		ID:        id,
		CreatedAt: imageConf.Created,
	}, nil
}

func (a *Remote) ManifestFromV1(id image.Ref) (image.Info, error) {
	history, err := a.Registry.Manifest(id.Repository(), id.Tag)
	if err != nil || history == nil {
		return image.Info{}, errors.Wrap(err, "getting remote manifest")
	}

	// the manifest includes some v1-backwards-compatibility data,
	// oddly called "History", which are layer metadata as JSON
	// strings; these appear most-recent (i.e., topmost layer) first,
	// so happily we can just decode the first entry to get a created
	// time.
	type v1image struct {
		Created time.Time `json:"created"`
	}
	var topmost v1image
	var img image.Info
	img.ID = id
	if len(history) > 0 {
		if err = json.Unmarshal([]byte(history[0].V1Compatibility), &topmost); err == nil {
			if !topmost.Created.IsZero() {
				img.CreatedAt = topmost.Created
			}
		}
	}

	return img, nil
}

// Cancel the remote request
func (a *Remote) Cancel() {
	a.CancelFunc()
}

// ---

// Cache is a local cache of image metadata.
type Cache struct {
	Reader cache.Reader
}

// GetRepository returns the list of image manifests in an image
// repository (e.g,. at "quay.io/weaveworks/flux")
func (c *Cache) GetRepository(id image.Name) ([]image.Info, error) {
	tags, err := c.Tags(id)
	if err != nil {
		return nil, err
	}
	return c.tagsToRepository(id, tags)
}

// GetImage gets the manifest of a specific image ref, from its
// registry.
func (c *Cache) GetImage(id image.Ref) (image.Info, error) {
	img, err := c.Manifest(id)
	if err != nil {
		return image.Info{}, err
	}
	return img, nil
}

func (c *Cache) tagsToRepository(id image.Name, tags []string) ([]image.Info, error) {
	type result struct {
		image image.Info
		err   error
	}

	toFetch := make(chan string, len(tags))
	fetched := make(chan result, len(tags))

	for i := 0; i < 100; i++ {
		go func() {
			for tag := range toFetch {
				image, err := c.Manifest(id.ToRef(tag))
				fetched <- result{image, err}
			}
		}()
	}
	for _, tag := range tags {
		toFetch <- tag
	}
	close(toFetch)

	images := make([]image.Info, cap(fetched))
	for i := 0; i < cap(fetched); i++ {
		res := <-fetched
		if res.err != nil {
			return nil, res.err
		}
		images[i] = res.image
	}

	sort.Sort(image.ByCreatedDesc(images))
	return images, nil
}

func (c *Cache) Manifest(id image.Ref) (image.Info, error) {
	key, err := cache.NewManifestKey(id.CanonicalRef())
	if err != nil {
		return image.Info{}, err
	}
	val, err := c.Reader.GetKey(key)
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

func (c *Cache) Tags(id image.Name) ([]string, error) {
	key, err := cache.NewTagKey(id.CanonicalName())
	if err != nil {
		return []string{}, err
	}
	val, err := c.Reader.GetKey(key)
	if err != nil {
		return []string{}, err
	}
	var tags []string
	err = json.Unmarshal(val, &tags)
	if err != nil {
		return []string{}, err
	}
	return tags, nil
}
