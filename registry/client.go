package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/registry/cache"
	"time"
)

// A client represents an entity that returns manifest and tags information.
// It might be a chache, it might be a real registry.
type Client interface {
	Tags(repository Repository) ([]string, error)
	Manifest(repository Repository, tag string) (flux.Image, error)
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
func (a *Remote) Tags(repository Repository) ([]string, error) {
	return a.Registry.Tags(repository.NamespaceImage())
}

// We need to do some adapting here to convert from the return values
// from dockerregistry to our domain types.
func (a *Remote) Manifest(repository Repository, tag string) (flux.Image, error) {
	img, err := flux.ParseImage(fmt.Sprintf("%s:%s", repository.String(), tag), time.Time{})
	if err != nil {
		return flux.Image{}, err
	}

	history, err := a.Registry.Manifest(repository.NamespaceImage(), tag)
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

func (c *Cache) Manifest(repository Repository, tag string) (flux.Image, error) {
	creds := c.creds.credsFor(repository.Host())
	key, err := cache.NewManifestKey(creds.username, repository.String(), tag)
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

func (c *Cache) Tags(repository Repository) ([]string, error) {
	creds := c.creds.credsFor(repository.Host())
	key, err := cache.NewTagKey(creds.username, repository.String())
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
