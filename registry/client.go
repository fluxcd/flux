package registry

import (
	"context"
	"encoding/json"
	"fmt"
	officialMemcache "github.com/bradfitz/gomemcache/memcache"
	"github.com/go-kit/kit/log"
	wraperrors "github.com/pkg/errors"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/registry/memcache"
	"strings"
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
		return flux.Image{}, err
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
	Client memcache.MemcacheClient
	logger log.Logger
}

func (*Cache) Cancel() {
	return
}

func NewCache(creds Credentials, cache memcache.MemcacheClient, expiry time.Duration, logger log.Logger) Client {
	return &Cache{
		creds:  creds,
		expiry: expiry,
		Client: cache,
		logger: logger,
	}
}

func (c *Cache) Manifest(repository Repository, tag string) (flux.Image, error) {
	img, err := flux.ParseImage(fmt.Sprintf("%s:%s", repository.String(), tag), time.Time{})
	if err != nil {
		return flux.Image{}, err
	}

	// Try the cache
	creds := c.creds.credsFor(repository.Host())
	key := manifestKey(creds.username, repository.String(), tag)
	cacheItem, err := c.Client.Get(key)
	if err != nil {
		if err != officialMemcache.ErrCacheMiss {
			c.logger.Log("err", wraperrors.Wrap(err, "Fetching tag from memcache"))
		}
		return flux.Image{}, err
	}
	err = json.Unmarshal(cacheItem.Value, &img)
	if err != nil {
		c.logger.Log("err", err.Error)
		return flux.Image{}, err
	}

	return img, nil
}

func (c *Cache) Tags(repository Repository) (tags []string, err error) {
	repo, err := ParseRepository(repository.String())
	if err != nil {
		c.logger.Log("err", wraperrors.Wrap(err, "Parsing repository"))
		return
	}
	creds := c.creds.credsFor(repo.Host())

	// Try the cache
	key := tagKey(creds.username, repo.String())
	cacheItem, err := c.Client.Get(key)
	if err != nil {
		if err != officialMemcache.ErrCacheMiss {
			c.logger.Log("err", wraperrors.Wrap(err, "Fetching tag from memcache"))
		}
		return
	}

	// Return the cache item
	err = json.Unmarshal(cacheItem.Value, &tags)
	if err != nil {
		c.logger.Log("err", err.Error)
		return
	}
	return
}

func manifestKey(username, repository, reference string) string {
	return strings.Join([]string{
		"registryhistoryv1", // Just to version in case we need to change format later.
		// Just the username here means we won't invalidate the cache when user
		// changes password, but that should be rare. And, it also means we're not
		// putting user passwords in plaintext into memcache.
		username,
		repository,
		reference,
	}, "|")
}

func tagKey(username, repository string) string {
	return strings.Join([]string{
		"registrytagsv1", // Just to version in case we need to change format later.
		// Just the username here means we won't invalidate the cache when user
		// changes password, but that should be rare. And, it also means we're not
		// putting user passwords in plaintext into memcache.
		username,
		repository,
	}, "|")
}
