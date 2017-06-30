package registry

import (
	"context"
	"encoding/json"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/registry/cache"
	"time"
)

// A client represents an entity that returns manifest and tags information.
// It might be a chache, it might be a real registry.
type Client interface {
	Tags(id flux.ImageID) ([]string, error)
	Manifest(id flux.ImageID, tag string) (flux.Image, error)
	Cancel()
}

// ---

// An implementation of Client that represents a Remote registry.
// E.g. docker hub.
type Remote struct {
	Registry   HerokuRegistryLibrary
	CancelFunc context.CancelFunc
}

// Return the tags for this repository.
func (a *Remote) Tags(id flux.ImageID) ([]string, error) {
	return a.Registry.Tags(id.NamespaceImage())
}

// We need to do some adapting here to convert from the return values
// from dockerregistry to our domain types.
func (a *Remote) Manifest(id flux.ImageID, tag string) (flux.Image, error) {
	history, err := a.Registry.Manifest(id.NamespaceImage(), tag)
	if err != nil || history == nil {
		return flux.Image{}, errors.Wrap(err, "getting remote manifest")
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
	var img flux.Image
	img.ID = id
	img.ID.Tag = tag
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

func (c *Cache) Manifest(id flux.ImageID, tag string) (flux.Image, error) {
	creds := c.creds.credsFor(id.Host)
	key, err := cache.NewManifestKey(creds.username, id.HostNamespaceImage(), tag)
	if err != nil {
		return flux.Image{}, err
	}
	val, err := c.cr.GetKey(key)
	if err != nil {
		return flux.Image{}, err
	}
	var img flux.Image
	err = json.Unmarshal(val, &img)
	if err != nil {
		c.logger.Log("err", err.Error)
		return flux.Image{}, err
	}
	return img, nil
}

func (c *Cache) Tags(id flux.ImageID) ([]string, error) {
	creds := c.creds.credsFor(id.Host)
	key, err := cache.NewTagKey(creds.username, id.HostNamespaceImage())
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
