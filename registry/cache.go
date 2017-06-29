package registry

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

type Cache struct {
	creds  Credentials
	expiry time.Duration
	Client MemcacheClient
	logger log.Logger
}

func NewCache(creds Credentials, cache MemcacheClient, expiry time.Duration, logger log.Logger) dockerRegistryInterface {
	return &Cache{
		creds:  creds,
		expiry: expiry,
		Client: cache,
		logger: logger,
	}
}

func (c *Cache) Manifest(repository, reference string) (history []schema1.History, err error) {
	repo, err := ParseRepository(repository)
	if err != nil {
		c.logger.Log("err", errors.Wrap(err, "Parsing repository"))
		return
	}
	creds := c.creds.credsFor(repo.Host())

	// Try the cache
	key := manifestKey(creds.username, repo.String(), reference)
	cacheItem, err := c.Client.Get(key)
	if err != nil {
		if err != memcache.ErrCacheMiss {
			c.logger.Log("err", errors.Wrap(err, "Fetching tag from memcache"))
		}
		return
	}

	// Return the cache item
	err = json.Unmarshal(cacheItem.Value, &history)
	if err != nil {
		c.logger.Log("err", err.Error)
		return
	}
	return
}

func (c *Cache) Tags(repository string) (tags []string, err error) {
	repo, err := ParseRepository(repository)
	if err != nil {
		c.logger.Log("err", errors.Wrap(err, "Parsing repository"))
		return
	}
	creds := c.creds.credsFor(repo.Host())

	// Try the cache
	key := tagKey(creds.username, repo.String())
	cacheItem, err := c.Client.Get(key)
	if err != nil {
		if err != memcache.ErrCacheMiss {
			c.logger.Log("err", errors.Wrap(err, "Fetching tag from memcache"))
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
