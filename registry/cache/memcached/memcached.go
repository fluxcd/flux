package memcached

import (
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/registry/cache"
)

const (
	expiry = time.Hour
)

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
	MaxIdleConns   int
}

func NewMemcacheClient(config MemcacheConfig) cache.Client {
	var servers memcache.ServerList
	client := memcache.NewFromSelector(&servers)
	client.Timeout = config.Timeout
	client.MaxIdleConns = config.MaxIdleConns

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
func NewFixedServerMemcacheClient(config MemcacheConfig, addresses ...string) cache.Client {
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

	return newClient
}

func (c *memcacheClient) GetKey(k cache.Keyer) ([]byte, time.Time, error) {
	cacheItem, err := c.Get(k.Key())
	if err != nil {
		if err == memcache.ErrCacheMiss {
			// Don't log on cache miss
			return []byte{}, time.Time{}, cache.ErrNotCached
		} else {
			c.logger.Log("err", errors.Wrap(err, "Fetching tag from memcache"))
			return []byte{}, time.Time{}, err
		}
	}
	expiry := binary.BigEndian.Uint32(cacheItem.Value)
	return cacheItem.Value[4:], time.Unix(int64(expiry), 0), nil
}

func (c *memcacheClient) SetKey(k cache.Keyer, v []byte) error {
	expiry := time.Now().Add(expiry).Unix()
	expiryBytes := make([]byte, 4, 4)
	binary.BigEndian.PutUint32(expiryBytes, uint32(expiry))
	if err := c.Set(&memcache.Item{
		Key:        k.Key(),
		Value:      append(expiryBytes, v...),
		Expiration: int32(expiry),
	}); err != nil {
		c.logger.Log("err", errors.Wrap(err, "storing in memcache"))
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
