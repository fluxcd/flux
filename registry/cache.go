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
	next   dockerRegistryInterface
	creds  Credentials
	expiry time.Duration
	Client MemcacheClient
	logger log.Logger
}

type CachedDockerRegistry func(dockerRegistryInterface) dockerRegistryInterface

func NewCache(creds Credentials, cache MemcacheClient, expiry time.Duration, logger log.Logger) CachedDockerRegistry {
	return func(next dockerRegistryInterface) dockerRegistryInterface {
		return &Cache{
			next:   next,
			creds:  creds,
			expiry: expiry,
			Client: cache,
			logger: logger,
		}
	}
}

func (c *Cache) Manifest(repository, reference string) ([]schema1.History, error) {
	// Don't cache latest. There are probably some other frequently changing tags
	// we shouldn't cache here as well.
	if reference == "latest" {
		return c.next.Manifest(repository, reference)
	}
	repo, err := ParseRepository(repository)
	if err != nil {
		return []schema1.History{}, err
	}
	creds := c.creds.credsFor(repo.Host())

	// Try the cache
	key := strings.Join([]string{
		"registryhistoryv1", // Just to version in case we need to change format later.
		// Just the username here means we won't invalidate the cache when user
		// changes password, but that should be rare. And, it also means we're not
		// putting user passwords in plaintext into memcache.
		creds.username,
		repository,
		reference,
	}, "|")
	cacheItem, err := c.Client.Get(key)
	if err == nil {
		// Return the cache item
		var history []schema1.History
		if err := json.Unmarshal(cacheItem.Value, &history); err == nil {
			return history, nil
		} else {
			c.logger.Log("err", err.Error)
		}
	} else if err != memcache.ErrCacheMiss {
		c.logger.Log("err", errors.Wrap(err, "Fetching tag from memcache"))
	}

	// fall back to the backend
	history, err := c.next.Manifest(repository, reference)
	if err == nil {
		// Store positive responses in the cache
		val, err := json.Marshal(history)
		if err != nil {
			c.logger.Log("err", errors.Wrap(err, "serializing tag to store in memcache"))
			return history, nil
		}
		if err := c.Client.Set(&memcache.Item{
			Key:        key,
			Value:      val,
			Expiration: int32(c.expiry.Seconds()),
		}); err != nil {
			c.logger.Log("err", errors.Wrap(err, "storing tag in memcache"))
			return history, nil
		}
	}

	return history, nil
}

// Pass through. Not caching tags.
func (c *Cache) Tags(repository string) ([]string, error) {
	return c.next.Tags(repository)
}
