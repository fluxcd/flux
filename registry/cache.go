package registry

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
)

type Cache struct {
	backend
	creds  Credentials
	expiry time.Duration
	Client *MemcacheClient
	logger log.Logger
}

// TODO: Add timing metrics
func NewCache(b backend, creds Credentials, cache *MemcacheClient, expiry time.Duration, logger log.Logger) *Cache {
	return &Cache{
		backend: b,
		creds:   creds,
		expiry:  expiry,
		Client:  cache,
		logger:  logger,
	}
}

func (c *Cache) Manifest(repository, reference string) (*schema1.SignedManifest, error) {
	// Don't cache latest. There are probably some other frequently changing tags
	// we shouldn't cache here as well.
	if reference == "latest" {
		return c.backend.Manifest(repository, reference)
	}

	host, _, _ := flux.ImageID(repository).Components()
	creds := c.creds.credsFor(host)

	// Try the cache
	key := strings.Join([]string{
		// Just the username here means we won't invalidate the cache when user
		// changes password, but that should be rare. And, it also means we're not
		// putting user passwords in plaintext into memcache.
		creds.username,
		repository,
		reference,
	}, "|")
	cacheItem, err := c.Client.Get(key)
	var m *schema1.SignedManifest
	if err == nil {
		// Return the cache item
		if err := json.Unmarshal(cacheItem.Value, m); err == nil {
			return m, nil
		} else {
			c.logger.Log("err", err.Error)
		}
	} else if err != memcache.ErrCacheMiss {
		// TODO: Log the error here.
	}

	// fall back to the backend
	m, err = c.backend.Manifest(repository, reference)
	if err == nil {
		// Store positive responses in the cache
		val, err := json.Marshal(m)
		if err != nil {
			c.logger.Log("err", err.Error)
			return m, nil
		}
		if err := c.Client.Set(&memcache.Item{
			Key:        key,
			Value:      val,
			Expiration: int32(c.expiry.Seconds()),
		}); err != nil {
			c.logger.Log("err", err.Error)
			return m, nil
		}
	}

	return m, nil
}
