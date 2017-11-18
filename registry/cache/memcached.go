package cache

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/image"
)

const (
	expiry = time.Hour
)

var (
	ErrNotCached = &fluxerr.Error{
		Type: fluxerr.Missing,
		Err:  memcache.ErrCacheMiss,
		Help: `Image not yet cached

It takes time to initially cache all the images. Please wait.

If you have waited for a long time, check the flux logs. Potential
reasons for the error are: no internet, no cache, error with the remote
repository.
`,
	}
)

type Reader interface {
	GetKey(k Keyer) ([]byte, error)
	GetExpiration(k Keyer) (time.Time, error)
}

type Writer interface {
	SetKey(k Keyer, v []byte) error
}

type Client interface {
	Reader
	Writer
	Stop()
}

type expiryData struct {
	Data   []byte
	Expiry int32
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
	MaxIdleConns   int
}

func NewMemcacheClient(config MemcacheConfig) Client {
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

func (c *memcacheClient) GetKey(k Keyer) ([]byte, error) {
	cacheItem, err := c.Get(k.Key())
	if err != nil {
		if err == memcache.ErrCacheMiss {
			// Don't log on cache miss
			return []byte{}, ErrNotCached
		} else {
			c.logger.Log("err", errors.Wrap(err, "Fetching tag from memcache"))
			return []byte{}, err
		}
	}
	var data expiryData
	err = json.Unmarshal(cacheItem.Value, &data)
	if err != nil {
		return []byte{}, err
	}
	return data.Data, nil
}

// GetExpiration returns the expiry time of the key
func (c *memcacheClient) GetExpiration(k Keyer) (time.Time, error) {
	cacheItem, err := c.Get(k.Key())
	if err != nil {
		if err == memcache.ErrCacheMiss {
			// Don't log on cache miss
			return time.Time{}, ErrNotCached
		} else {
			c.logger.Log("err", errors.Wrap(err, "Fetching tag from memcache"))
			return time.Time{}, err
		}
	}

	var data expiryData
	err = json.Unmarshal(cacheItem.Value, &data)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(int64(data.Expiry), 0), nil
}

func (c *memcacheClient) SetKey(k Keyer, v []byte) error {
	expiry := int32(time.Now().Add(expiry).Unix())
	data := expiryData{
		Expiry: expiry,
		Data:   v,
	}
	val, err := json.Marshal(&data)
	if err != nil {
		return err
	}
	if err := c.Set(&memcache.Item{
		Key:        k.Key(),
		Value:      val,
		Expiration: expiry,
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

// An interface to provide the key under which to store the data
// Use the full path to image for the memcache key because there
// might be duplicates from other registries
type Keyer interface {
	Key() string
}

type manifestKey struct {
	fullRepositoryPath, reference string
}

func NewManifestKey(image image.CanonicalRef) (Keyer, error) {
	return &manifestKey{image.CanonicalName().String(), image.Tag}, nil
}

func (k *manifestKey) Key() string {
	return strings.Join([]string{
		"registryhistoryv3", // Just to version in case we need to change format later.
		k.fullRepositoryPath,
		k.reference,
	}, "|")
}

type tagKey struct {
	fullRepositoryPath string
}

func NewTagKey(id image.CanonicalName) (Keyer, error) {
	return &tagKey{id.String()}, nil
}

func (k *tagKey) Key() string {
	return strings.Join([]string{
		"registrytagsv3", // Just to version in case we need to change format later.
		k.fullRepositoryPath,
	}, "|")
}
