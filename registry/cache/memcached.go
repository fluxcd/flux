package cache

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/weaveworks/flux"
	"strings"
)

const expiry = time.Hour

// Interface to read from a cache
// fullRepo must be the full path to the image, including host. E.g.
// "index.docker.io/weaveworks/foorepo" otherwise it will cache in
// the wrong location.
type Reader interface {
	GetKey(k Key) ([]byte, error)
}

type Writer interface {
	SetKey(k Key, v []byte) error
}

type Client interface {
	Reader
	Writer
	Stop()
}

// MemcacheClient is a memcache client that gets its server list from SRV
// records, and periodically updates that ServerList.
type memcacheClient struct {
	*memcache.Client
	serverList *memcache.ServerList
	hostname   string
	service    string
	logger     log.Logger

	quit chan struct{}
	wait sync.WaitGroup
}

// MemcacheConfig defines how a MemcacheClient should be constructed.
type MemcacheConfig struct {
	Host           string
	Service        string
	Timeout        time.Duration
	UpdateInterval time.Duration
	Logger         log.Logger
}

func NewMemcacheClient(config MemcacheConfig) Client {
	var servers memcache.ServerList
	client := memcache.NewFromSelector(&servers)
	client.Timeout = config.Timeout

	newClient := &memcacheClient{
		Client:     client,
		serverList: &servers,
		hostname:   config.Host,
		service:    config.Service,
		logger:     config.Logger,
		quit:       make(chan struct{}),
	}

	err := newClient.updateMemcacheServers()
	if err != nil {
		config.Logger.Log("err", errors.Wrapf(err, "Error setting memcache servers to '%v'", config.Host))
	}

	newClient.wait.Add(1)
	go newClient.updateLoop(config.UpdateInterval)
	return newClient
}

// Does not use DNS, accepts static list of servers.
func NewFixedServerMemcacheClient(config MemcacheConfig, addresses ...string) Client {
	var servers memcache.ServerList
	servers.SetServers(addresses...)
	client := memcache.NewFromSelector(&servers)
	client.Timeout = config.Timeout

	newClient := &memcacheClient{
		Client:     client,
		serverList: &servers,
		hostname:   config.Host,
		service:    config.Service,
		logger:     config.Logger,
		quit:       make(chan struct{}),
	}

	client.FlushAll()
	return newClient
}

func (c *memcacheClient) GetKey(k Key) ([]byte, error) {
	cacheItem, err := c.Get(k.String())
	if err != nil {
		if err == memcache.ErrCacheMiss {
			// Don't log on cache miss
			return []byte{}, err
		} else {
			c.logger.Log("err", errors.Wrap(err, "Fetching tag from memcache"))
			return []byte{}, err
		}
	}
	return cacheItem.Value, nil
}

func (c *memcacheClient) SetKey(k Key, v []byte) error {
	if err := c.Set(&memcache.Item{
		Key:        k.String(),
		Value:      v,
		Expiration: int32(expiry.Seconds()),
	}); err != nil {
		c.logger.Log("err", errors.Wrap(err, "storing tags in memcache"))
		return err
	}
	return nil
}

// Stop the memcache client.
func (c *memcacheClient) Stop() {
	close(c.quit)
	c.wait.Wait()
}

func (c *memcacheClient) updateLoop(updateInterval time.Duration) {
	defer c.wait.Done()
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.updateMemcacheServers(); err != nil {
				c.logger.Log("err", errors.Wrap(err, "error updating memcache servers"))
			}
		case <-c.quit:
			return
		}
	}
}

// updateMemcacheServers sets a memcache server list from SRV records. SRV
// priority & weight are ignored.
func (c *memcacheClient) updateMemcacheServers() error {
	_, addrs, err := net.LookupSRV(c.service, "tcp", c.hostname)
	if err != nil {
		return err
	}
	var servers []string
	for _, srv := range addrs {
		servers = append(servers, fmt.Sprintf("%s:%d", srv.Target, srv.Port))
	}
	// ServerList deterministically maps keys to _index_ of the server list.
	// Since DNS returns records in different order each time, we sort to
	// guarantee best possible match between nodes.
	sort.Strings(servers)
	return c.serverList.SetServers(servers...)
}

// An interface to provide the key under which to store the data
// Use the full path to image for the memcache key because there
// might be duplicates from other registries
type Key interface {
	String() string
}

type manifestKey struct {
	username, fullRepositoryPath, reference string
}

func NewManifestKey(username string, id flux.ImageID) (Key, error) {
	return &manifestKey{username, id.HostNamespaceImage(), id.Tag}, nil
}

func (k *manifestKey) String() string {
	return strings.Join([]string{
		"registryhistoryv1", // Just to version in case we need to change format later.
		// Just the username here means we won't invalidate the cache when user
		// changes password, but that should be rare. And, it also means we're not
		// putting user passwords in plaintext into memcache.
		k.username,
		k.fullRepositoryPath,
		k.reference,
	}, "|")
}

type tagKey struct {
	username, fullRepositoryPath string
}

func NewTagKey(username string, id flux.ImageID) (Key, error) {
	return &tagKey{username, id.HostNamespaceImage()}, nil
}

func (k *tagKey) String() string {
	return strings.Join([]string{
		"registrytagsv1", // Just to version in case we need to change format later.
		// Just the username here means we won't invalidate the cache when user
		// changes password, but that should be rare. And, it also means we're not
		// putting user passwords in plaintext into memcache.
		k.username,
		k.fullRepositoryPath,
	}, "|")
}
