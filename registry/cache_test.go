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
	"github.com/docker/distribution/manifest/schema1"
	"github.com/go-kit/kit/log"
)

var (
	memcachedIPs = flag.String("memcached-ips", "127.0.0.1:11211", "space-separated host:port values for memcached to connect to")
)

type stoppableMemcacheClient struct {
	*memcache.Client
}

func (s *stoppableMemcacheClient) Stop() {}

// Setup sets up stuff for testing
func Setup(t *testing.T) MemcacheClient {
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

	val, _ := json.Marshal([]schema1.History{{`{"test":"json"}`}})
	key := manifestKey(creds.credsFor("").username, "weaveworks/foorepo", "tag1")
	if err := mc.Set(&memcache.Item{
		Key:        key,
		Value:      val,
		Expiration: int32(time.Hour.Seconds()),
	}); err != nil {
		t.Fatal(err)
	}

	// It should fetch stuff
	response, err := c.Manifest("weaveworks/foorepo", "tag1")
	if err != nil {
		t.Fatal(err)
	}
	if len(response) != 1 {
		t.Fatalf("Expected 1 history item, got %v", response)
	}
	expected := schema1.History{`{"test":"json"}`}
	if response[0] != expected {
		t.Fatalf("Expected  history item: %v, got %v", expected, response[0])
	}

	// It should miss if not in cache
	_, err = c.Manifest("weaveworks/anotherrepo", "tag1")
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

	val, _ := json.Marshal([]string{"tag1", "tag2"})
	key := tagKey(creds.credsFor("").username, "weaveworks/foorepo")
	if err := mc.Set(&memcache.Item{
		Key:        key,
		Value:      val,
		Expiration: int32(time.Hour.Seconds()),
	}); err != nil {
		t.Fatal(err)
	}

	// It should fetch stuff
	response, err := c.Tags("weaveworks/foorepo")
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

	// It should miss if not in cache
	_, err = c.Tags("weaveworks/anotherrepo")
	if err != memcache.ErrCacheMiss {
		t.Fatal("Expected cache miss")
	}
}
