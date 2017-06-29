// +build integration

package registry

import (
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"encoding/json"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/go-kit/kit/log"
	registryMemcache "github.com/weaveworks/flux/registry/memcache"
)

var (
	memcachedIPs = flag.String("memcached-ips", "127.0.0.1:11211", "space-separated host:port values for memcached to connect to")
)

type stoppableMemcacheClient struct {
	*memcache.Client
}

func (s *stoppableMemcacheClient) Stop() {}

// Setup sets up stuff for testing
func Setup(t *testing.T) registryMemcache.MemcacheClient {
	mc := memcache.New(strings.Fields(*memcachedIPs)...)
	if err := mc.FlushAll(); err != nil {
		t.Fatal(err)
	}
	return &stoppableMemcacheClient{mc}
}

func TestCache_Manifests(t *testing.T) {
	mc := Setup(t)
	defer mc.Stop()

	creds := NoCredentials()
	c := NewCache(
		creds,
		mc,
		20*time.Minute,
		log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
	)

	r, _ := ParseRepository("index.docker.io/weaveworks/foorepo")
	img := r.ToImage("tag1")
	val, _ := json.Marshal(img)
	key := manifestKey(creds.credsFor(r.Host()).username, r.String(), img.ID.Tag)
	if err := mc.Set(&memcache.Item{
		Key:        key,
		Value:      val,
		Expiration: int32(time.Hour.Seconds()),
	}); err != nil {
		t.Fatal(err)
	}

	// It should fetch stuff
	response, err := c.Manifest(r, img.ID.Tag)
	if err != nil {
		t.Fatal(err)
	}
	if response.ID.String() == "" {
		t.Fatal("Should have returned image")
	}

	r2, _ := ParseRepository("index.docker.io/weaveworks/another")
	// It should miss if not in cache
	_, err = c.Manifest(r2, "tag1")
	if err != memcache.ErrCacheMiss {
		t.Fatal("Expected cache miss")
	}
}

func TestCache_Tags(t *testing.T) {
	mc := Setup(t)

	creds := NoCredentials()
	c := NewCache(
		creds,
		mc,
		20*time.Minute,
		log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
	)

	r, _ := ParseRepository("index.docker.io/weaveworks/foorepo")
	val, _ := json.Marshal([]string{"tag1", "tag2"})
	key := tagKey(creds.credsFor(r.Host()).username, r.String())
	if err := mc.Set(&memcache.Item{
		Key:        key,
		Value:      val,
		Expiration: int32(time.Hour.Seconds()),
	}); err != nil {
		t.Fatal(err)
	}

	// It should fetch stuff
	response, err := c.Tags(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(response) != 2 {
		t.Fatalf("Expected 2 tags item, got %v", response)
	}
	expected := "tag1"
	if response[0] != expected {
		t.Fatalf("Expected  history item: %v, got %v", expected, response[0])
	}

	r2, _ := ParseRepository("index.docker.io/weaveworks/anotherrepo")
	// It should miss if not in cache
	_, err = c.Tags(r2)
	if err != memcache.ErrCacheMiss {
		t.Fatal("Expected cache miss")
	}
}
