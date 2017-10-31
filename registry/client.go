package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-kit/kit/log"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry/cache"
)

// A client represents an entity that returns manifest and tags
// information.  It might be a cache, it might be a real registry.
type Client interface {
	Tags(name image.Name) ([]string, error)
	Manifest(name image.Ref) (image.Info, error)
	Cancel()
}

// ---

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

// An implementation of Client backed by Memcache
type Cache struct {
	creds  Credentials
	expiry time.Duration
	cr     cache.Reader
	logger log.Logger
}

func (*Cache) Cancel() {
	return
}

func NewCache(creds Credentials, cr cache.Reader, expiry time.Duration, logger log.Logger) Client {
	return &Cache{
		creds:  creds,
		expiry: expiry,
		cr:     cr,
		logger: logger,
	}
}

func (c *Cache) Manifest(id image.Ref) (image.Info, error) {
	creds := c.creds.credsFor(id.Registry())
	key, err := cache.NewManifestKey(creds.username, id.CanonicalRef())
	if err != nil {
		return image.Info{}, err
	}
	val, err := c.cr.GetKey(key)
	if err != nil {
		return image.Info{}, err
	}
	var img image.Info
	err = json.Unmarshal(val, &img)
	if err != nil {
		c.logger.Log("err", err.Error)
		return image.Info{}, err
	}
	return img, nil
}

func (c *Cache) Tags(id image.Name) ([]string, error) {
	creds := c.creds.credsFor(id.Registry())
	key, err := cache.NewTagKey(creds.username, id.CanonicalName())
	if err != nil {
		return []string{}, err
	}
	val, err := c.cr.GetKey(key)
	if err != nil {
		return []string{}, err
	}
	var tags []string
	err = json.Unmarshal(val, &tags)
	if err != nil {
		c.logger.Log("err", err.Error)
		return []string{}, err
	}
	return tags, nil
}
