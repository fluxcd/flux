/* This package implements an image DB cache using memcached.

Items are given an expiry based on their refresh deadline, with a
minimum duration to try and ensure things will expire well after they
would have been refreshed (i.e., only if they truly need garbage
collection).

memcached will still evict things when under memory pressure. We can
recover from that -- we'll just get a cache miss, and fetch it again.

*/
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

	"github.com/fluxcd/flux/pkg/registry/cache"
)

const (
	// The minimum expiry given to an entry.
	MinExpiry = time.Hour
)

// MemcacheClient is a memcache client that gets its server list from SRV
// records, and periodically updates that ServerList.
type MemcacheClient struct {
	client     *memcache.Client
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

func NewMemcacheClient(config MemcacheConfig) *MemcacheClient {
	var servers memcache.ServerList
	client := memcache.NewFromSelector(&servers)
	client.Timeout = config.Timeout
	client.MaxIdleConns = config.MaxIdleConns

	newClient := &MemcacheClient{
		client:     client,
		serverList: &servers,
		hostname:   config.Host,
		service:    config.Service,
		logger:     config.Logger,
		quit:       make(chan struct{}),
	}

	err := newClient.updateFromSRVRecords()
	if err != nil {
		config.Logger.Log("err", errors.Wrapf(err, "Error setting memcache servers to '%v'", config.Host))
	}

	newClient.wait.Add(1)
	go newClient.updateLoop(config.UpdateInterval, newClient.updateFromSRVRecords)
	return newClient
}

// Does not use DNS, accepts static list of servers.
func NewFixedServerMemcacheClient(config MemcacheConfig, addresses ...string) *MemcacheClient {
	var servers memcache.ServerList
	servers.SetServers(addresses...)
	client := memcache.NewFromSelector(&servers)
	client.Timeout = config.Timeout

	newClient := &MemcacheClient{
		client:     client,
		serverList: &servers,
		hostname:   config.Host,
		service:    config.Service,
		logger:     config.Logger,
		quit:       make(chan struct{}),
	}

	go newClient.updateLoop(config.UpdateInterval, func() error {
		return servers.SetServers(addresses...)
	})
	return newClient
}

// GetKey gets the value and its refresh deadline from the cache.
func (c *MemcacheClient) GetKey(k cache.Keyer) ([]byte, time.Time, error) {
	cacheItem, err := c.client.Get(k.Key())
	if err != nil {
		if err == memcache.ErrCacheMiss {
			// Don't log on cache miss
			return []byte{}, time.Time{}, cache.ErrNotCached
		} else {
			c.logger.Log("err", errors.Wrap(err, "Fetching tag from memcache"))
			return []byte{}, time.Time{}, err
		}
	}
	deadlineTime := binary.BigEndian.Uint32(cacheItem.Value)
	return cacheItem.Value[4:], time.Unix(int64(deadlineTime), 0), nil
}

// SetKey sets the value and its refresh deadline at a key. NB the key
// expiry is set _longer_ than the deadline, to give us a grace period
// in which to refresh the value.
func (c *MemcacheClient) SetKey(k cache.Keyer, refreshDeadline time.Time, v []byte) error {
	expiry := refreshDeadline.Sub(time.Now()) * 2
	if expiry < MinExpiry {
		expiry = MinExpiry
	}

	deadlineBytes := make([]byte, 4, 4)
	binary.BigEndian.PutUint32(deadlineBytes, uint32(refreshDeadline.Unix()))
	if err := c.client.Set(&memcache.Item{
		Key:        k.Key(),
		Value:      append(deadlineBytes, v...),
		Expiration: int32(expiry.Seconds()),
	}); err != nil {
		c.logger.Log("err", errors.Wrap(err, "storing in memcache"))
		return err
	}
	return nil
}

// Stop the memcache client.
func (c *MemcacheClient) Stop() {
	close(c.quit)
	c.wait.Wait()
}

func (c *MemcacheClient) updateLoop(updateInterval time.Duration, update func() error) {
	defer c.wait.Done()
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := update(); err != nil {
				c.logger.Log("err", errors.Wrap(err, "error updating memcache servers"))
			}
		case <-c.quit:
			return
		}
	}
}

// updateMemcacheServers sets a memcache server list from SRV records. SRV
// priority & weight are ignored.
func (c *MemcacheClient) updateFromSRVRecords() error {
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
