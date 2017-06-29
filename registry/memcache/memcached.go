package memcache

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

type MemcacheClient interface {
	Add(item *memcache.Item) error
	CompareAndSwap(item *memcache.Item) error
	Decrement(key string, delta uint64) (newValue uint64, err error)
	Delete(key string) error
	DeleteAll() error
	FlushAll() error
	Get(key string) (item *memcache.Item, err error)
	GetMulti(keys []string) (map[string]*memcache.Item, error)
	Increment(key string, delta uint64) (newValue uint64, err error)
	Replace(item *memcache.Item) error
	Set(item *memcache.Item) error
	Touch(key string, seconds int32) (err error)

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

func NewMemcacheClient(config MemcacheConfig) MemcacheClient {
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
